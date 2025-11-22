(ns vector-db-process
  (:require
   [babashka.process :as process]
   [cheshire.core :as json]
   [clojure.core.async :as async]
   [clojure.java.io :as io]
   [lsp4clj.io-server :as io-server]
   [lsp4clj.server :as server]
   [lsp4clj.io-chan :as io-chan]))

;; Start the vector DB docker container as a background process
;; and return the process handle with stdin/stdout/stderr streams
(defn start-vector-db
  "Start the jimclark106/vector-db Docker container with interactive streams.
   Returns a map with :process, :in (stdin stream), :out (stdout stream), and :err (stderr stream)"
  [{:keys [dimension db]}]
  (let [cmd ["docker" "run" "-i" "--rm"
             "--platform" "linux/amd64"
             "-v" "./data:/data"
             "-e" (format "DB_PATH=/data/%s" db)
             "-e" (format "VECTOR_DIMENSION=%s" dimension)
             "jimclark106/vector-db:latest"]
        proc (process/process cmd {:in :stream
                                   :out :stream
                                   :err :stream})]
    {:process proc
     :in (:in proc)
     :out (:out proc)
     :err (:err proc)}))

(defn stop-container
  "Stop the container by destroying the process"
  [{:keys [process]}]
  (process/destroy process))

(defn container-alive?
  "Check if the container process is still alive"
  [{:keys [process]}]
  (process/alive? process))

(defn wait-for-container
  "Wait for the container to exit and return the exit code"
  [{:keys [process]}]
  @process)

(declare mcp-initialize)
(defn vector-db-stdio-server
  "Create a stdio-server using the Docker container's stdin/stdout streams.
   First starts the vector-db container, then creates a server reading from
   the container's stdout and writing to its stdin.
   Returns a map with :server, :container, and :join (future that completes when server exits)."
  ([] (vector-db-stdio-server {:dimension 1536 :db "vectors.db"}))
  ([opts]
   (let [log-ch (or (:log-ch opts) (async/chan))
         trace-ch (or (:trace-ch opts) (async/chan))
         container (start-vector-db opts)

         ;; Debug: spawn a thread to monitor stderr
         _ (async/thread
             (let [reader (io/reader (:err container))]
               (loop []
                 (when-let [_ (.readLine reader)]
                   (recur)))))

         ;; Use keyword instead of csk/->kebab-case-keyword to keep keys as-is
         ;; The lsp4clj server expects :id, :jsonrpc, :method, :result, etc.
         mcp-in-factory (fn [in opts]
                          (io-chan/mcp-input-stream->input-chan in (assoc opts
                                                                          :keyword-function keyword
                                                                          :log-ch log-ch)))
         srv (io-server/server (merge {:trace-level "verbose"}
                                      opts
                                      {:in (:out container)
                                       :out (:in container)
                                       :log-ch log-ch
                                       ;:trace-ch trace-ch
                                       :in-chan-factory mcp-in-factory
                                       :out-chan-factory io-chan/mcp-output-stream->output-chan}))
         join (server/start srv nil)]
     ;; Spawn a thread to print log messages (first 20 chars only)
     (async/go-loop []
       (when-let [log-msg (async/<! log-ch)]
         (let [msg-str (str log-msg)]
           (println "LOG:" (subs msg-str 0 (min 20 (count msg-str)))))
         (recur)))
     (async/go-loop []
       (when-let [trace-msg (async/<! trace-ch)]
         (let [msg-str (str trace-msg)]
           (println "TRACE:" (subs msg-str 0 (min 20 (count msg-str)))))
         (recur)))
     (let [m {:server srv
              :container container
              :join join}]
       (async/<!! (mcp-initialize m))
       (server/send-notification (:server m) "notifications/initialized" {})
       m))))

(defn mcp-initialize
  "Send an MCP initialize request to the vector-db server.
   Takes the return value from vector-db-stdio-server as first argument.
   Returns a go channel that will emit the initialize response when ready."
  [{:keys [server]} & [params]]
  (let [result-ch (async/go
                    (let [req (server/send-request server "initialize" (or params {}))
                          resp (server/deref-or-cancel req 10000 :timeout)]
                      (if (= :timeout resp)
                        {:error "Initialize request timed out"}
                        resp)))]
    result-ch))

(defn mcp-call-tool
  "Call a tool on the MCP server.
   Takes the return value from vector-db-stdio-server as first argument,
   the tool name as second argument, and optional arguments map as third argument.
   Returns a go channel that will emit the tool response when ready."
  [{:keys [server]} tool-name & [arguments]]
  (let [result-ch (async/go
                    (let [req (server/send-request server "tools/call"
                                                   {:name tool-name
                                                    :arguments (or arguments {})})
                          resp (server/deref-or-cancel req 30000 :timeout)]
                      (if (= :timeout resp)
                        {:error (str "Tool call '" tool-name "' timed out")}
                        resp)))]
    result-ch))

(defn mcp-list-tools
  "List available tools from the MCP server.
   Takes the return value from vector-db-stdio-server as first argument.
   Returns a go channel that will emit the list of tools when ready."
  [{:keys [server]} & [params]]
  (let [result-ch (async/go
                    (let [req (server/send-request server "tools/list" (or params {}))
                          resp (server/deref-or-cancel req 5000 :timeout)]
                      (if (= :timeout resp)
                        {:error "List tools request timed out"}
                        resp)))]
    result-ch))

;; ==================================================
;; Vector DB Tool Operations
;; ==================================================

(defn create-collection
  "Create a new vector collection"
  [server-container collection-name]
  (mcp-call-tool server-container "create_collection" {:name collection-name}))

(defn delete-collection
  "Delete a collection and all its vectors"
  [server-container collection-name]
  (mcp-call-tool server-container "delete_collection" {:name collection-name}))

(defn list-collections
  "List all vector collections in the database.
   Returns a go channel that will emit the parsed collection list."
  [server-container]
  (async/go
    (let [response (async/<! (mcp-call-tool server-container "list_collections" {}))]
      (if (:error response)
        response
        (try
          (-> response :content first :text (json/parse-string keyword))
          (catch Exception e
            {:error (str "Failed to parse collections response: " (.getMessage e))}))))))

(defn add-vector
  "Add a vector to a collection (creates collection if it doesn't exist).
   vector must be a sequence of 1536 numbers.
   metadata is an optional map."
  [server-container collection-name vector & [metadata]]
  (mcp-call-tool server-container "add_vector"
                 (cond-> {:collection_name collection-name
                          :vector vector}
                   metadata (assoc :metadata metadata))))

(defn delete-vector
  "Delete a vector by its ID"
  [server-container vector-id]
  (mcp-call-tool server-container "delete_vector" {:id vector-id}))

(defn search-vectors
  "Search for similar vectors using cosine distance.
   vector must be a sequence of 1536 numbers.
   Options:
   - :collection_name - search only within this collection
   - :exclude_collections - vector of collection names to exclude
   - :limit - maximum number of results (default 10)
   Returns a go channel that will emit the parsed search results."
  [server-container vector & [options]]
  (async/go
    (let [response (async/<! (mcp-call-tool server-container "search"
                                            (merge {:vector vector}
                                                   (select-keys options [:collection_name :exclude_collections :limit]))))]
      (if (:error response)
        response
        (try
          (-> response :content first :text (json/parse-string keyword))
          (catch Exception e
            {:error (str "Failed to parse search response: " (.getMessage e))}))))))

(comment
  ;; Start the container
  (def db (start-vector-db {:dimension 1536 :db "vectors.db"}))

  ;; Access the raw streams
  (:in db)   ; stdin stream
  (:out db)  ; stdout stream
  (:err db)  ; stderr stream

  ;; Check if it's running
  (container-alive? db)

  ;; Stop the container when done
  (stop-container db)

  ;; Or wait for it to exit naturally
  (wait-for-container db)

  ;; Create a stdio-server using the container's streams
  (def server-container (vector-db-stdio-server))
  (:server server-container)  ; The stdio-server
  (:container server-container)  ; The container info

  ;; Initialize the MCP connection
  (def init-ch (mcp-initialize server-container {:clientInfo {:name "test-client" :version "1.0.0"}}))
  ;; Wait for the response
  (async/<!! init-ch)

  ;; List available tools
  (def list-ch (mcp-list-tools server-container))
  ;; Wait for the tools list
  (async/<!! list-ch)

  ;; Call a tool on the server
  (def tool-ch (mcp-call-tool server-container "search" {:query "example query" :limit 10}))
  ;; Wait for the tool response
  (async/<!! tool-ch)

  ;; Using the vector-db wrapper functions:

  ;; Create a collection
  (async/<!! (create-collection server-container "my-collection"))

  ;; List collections
  (async/<!! (list-collections server-container))

  ;; Add a vector (1536 dimensions)
  (def sample-vector (vec (repeat 1536 0.1)))
  (async/<!! (add-vector server-container "my-collection" sample-vector {:name "test-doc"}))

  ;; Search for similar vectors
  (async/<!! (search-vectors server-container sample-vector {:collection_name "my-collection" :limit 5}))

  ;; Delete a vector by ID
  (async/<!! (delete-vector server-container 1))

  ;; Delete a collection
  (async/<!! (delete-collection server-container "my-collection"))

  ;; Stop the container when done
  (stop-container (:container server-container)))
