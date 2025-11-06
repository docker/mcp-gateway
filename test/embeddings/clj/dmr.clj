(ns dmr
  (:require
   [babashka.curl :as curl]
   [cheshire.core :as json]
   [clj-yaml.core :as yaml]
   [clojure.core.async :as async]
   [tolkien.core :as tolkien]
   [vector-db-process :as vec-db]))

;; ==================================================
;; DMR
;; ==================================================

(def embedding-model "ai/embeddinggemma:latest")
(def summary-model "ai/gemma3-qat:latest")
(def url "localhost/exp/vDD4.40/engines/llama.cpp/v1/embeddings")
(def models-url "localhost/exp/vDD4.40/engines/llama.cpp/v1/models")
(def create-models-url "localhost/exp/vDD4.40/models/create")
(def socket-path {:raw-args ["--unix-socket" "/var/run/docker.sock"]})
(def summary-url "localhost/exp/vDD4.40/engines/llama.cpp/v1/chat/completions")

(defn get-models-url [namespace name] (format "localhost/exp/vDD4.40/engines/llama.cpp/v1/models/%s/%s" namespace name))

(defn check
  "check the http response"
  [status response]
  (when (not (= status (:status response)))
    (println (format "%s not equal %s - %s" status (:status response) response))
    (throw (ex-info "failed" response)))
  response)

(defn dmr-embeddings
  "Stub function for /exp/vDD4.40/engines/llama.cpp/v1/chat/embeddings endpoint."
  [embedding-model request]
  (curl/post
   url
   (merge
    socket-path
    (update
     {:body {:model embedding-model}
      :headers {"Content-Type" "application/json"}
      :throw false}
     :body (comp json/generate-string merge) request))))

(defn dmr-completion
  "Stub function for /exp/vDD4.40/engines/llama.cpp/v1/chat/embeddings endpoint."
  [summary-model request]
  (curl/post
   summary-url
   (merge
    socket-path
    (update
     {:body {:model summary-model}
      :headers {"Content-Type" "application/json"}
      :throw false}
     :body (comp json/generate-string merge) request))))

(defn dmr-models []
  (curl/get
   models-url
   (merge
    socket-path
    {:throw false})))

(defn dmr-get-model [namespace name]
  (curl/get
   (get-models-url namespace name)
   (merge
    socket-path
    {:throw false})))

(defn dmr-create-model [s]
  (curl/post
   create-models-url
   (merge
    socket-path
    {:throw false
     :body (json/generate-string {:from s})})))

;; ==================================================
;; OpenAI
;; ==================================================
(defn gpt-embeddings
  [request]
  (curl/post
   "https://api.openai.com/v1/embeddings"
   (update
    {:body {:model "text-embedding-3-small"}
     :headers {"Content-Type" "application/json"
               "Authorization" (format "Bearer %s" (System/getenv "OPENAI_API_KEY"))}
     :throw false}
    :body (comp json/generate-string merge) request)))

(defn gpt-completion
  [request]
  (curl/post
   "https://api.openai.com/v1/chat/completions"
   (update
    {:body {:model "gpt-4.1"}
     :headers {"Content-Type" "application/json"
               "Authorization" (format "Bearer %s" (System/getenv "OPENAI_API_KEY"))}
     :throw false}
    :body (comp json/generate-string merge) request)))

;; ==================================================
;; LLM Ops that could work with either OpenAI or DMR
;; ==================================================
(defn create-embedding [embedding-fn s]
  (->
   ((comp (partial check 200) embedding-fn) {:input s})
   :body
   (json/parse-string keyword)
   :data
   first
   :embedding))

(defn summarize-tool [completion-fn s]
  (->
   ((comp (partial check 200) completion-fn)
    {:messages
     [{:role "user"
       :content (format
                 "Summarize the following content thoroughly but remove any examples or extraneous details 
                  Do not try to explain how you summarized or that you're providing a summary.  
                  Always return a summary.  Do not just return the input json.
                  Start summarizing everything coming after this: \n\n%s" s)}]})
   :body
   (json/parse-string keyword)
   :choices
   first
   :message
   :content))

;; ==================================================
;; Vector DB OPs
;; ==================================================
;(ns-unmap *ns* 'vec-db-connection)
(defonce vec-db-connection (vec-db/vector-db-stdio-server {:dimension 2560}))

(defn search [{:keys [embedding-fn] :as options} s]
  (let [vec (create-embedding embedding-fn s)]
    (vec-db/search-vectors vec-db-connection vec options)))

;; ==================================================
;; Perform Embeddings
;; ==================================================
(defn summarize-registration [registration]
  (str
   #_(format "This tool comes from %s\n%s\n" (:server_name registration) (:server_title registration))
   (format "It provides the tool %s %s - %s\n" (-> registration :tool :name) (or (-> registration :tool :title) "") (-> registration :tool :description))
   (format "Input parameters are %s" (->> registration
                                          :tool
                                          :inputSchema
                                          :properties
                                          (map (fn [[k v]] (format "%s %s\n" (name k) (:description v))))
                                          (apply str)))))

(defn summarize-tools [tool-registrations]
  (doseq [tool-registration tool-registrations]
    (println "-------" (-> tool-registration :tool :name) "--------" (count (json/generate-string tool-registration)))
    (println (try
               (summarize-tool
                (partial dmr-completion summary-model)
                (json/generate-string tool-registration))
               (catch Throwable _ "failed to summarize")))))

(defn embed-servers
  "embed the server descriptions"
  [{:keys [embedding-fn summarize-fn]} collection servers]
  (println "> embed " (:name collection))
  (async/go
    (async/<! (vec-db/delete-collection vec-db-connection (:name collection)))
    (async/<! (vec-db/create-collection vec-db-connection (:name collection)))
    (doseq [server servers :let [summary
                                 (summarize-fn server)]]
      (println "  > embed " (-> server :name) " -> " (count summary))
      (let [vec (create-embedding embedding-fn summary)]
        (async/<!! (vec-db/add-vector vec-db-connection (:name collection) vec (select-keys server [:name])))))))

(defn embed-server-tools
  "embed the server descriptions"
  [{:keys [embedding-fn summarize-fn]} collection tool-registrations]
  (println "> embed " (:name collection))
  (async/go
    (async/<! (vec-db/delete-collection vec-db-connection (:name collection)))
    (async/<! (vec-db/create-collection vec-db-connection (:name collection)))
    (doseq [tool-registration tool-registrations :let [summary (summarize-fn tool-registration)]]
      (let [vec (time (create-embedding embedding-fn summary))]
        (println "  > embed " (-> tool-registration :tool :name) " -> " (count summary))
        (async/<!! (vec-db/add-vector vec-db-connection (:name collection) vec {:tool (select-keys (:tool tool-registration) [:name])}))))))

(defn json-with-token-check [tool-registration]
  (let [json (json/generate-string tool-registration)]
    (if (< 2048 (tolkien/count-tokens "text-embedding-3-small" json))
      (-> tool-registration
          (update :tool dissoc :outputSchema)
          (json/generate-string))
      json)))

(def servers
  ["github-official" "gitmcp" "slack" "fetch" "duckduckgo"
   "brave" "context7" "dockerhub" "playwright" "wikipedia-mcp" "SQLite" "notion-remote" "rust-mcp-filesystem" "arxiv-mcp-server" "google-maps" "google-maps-comprehensive" "hugging-face" "linkedin-mcp-server" "desktop-commander"
   "openbnb-airbnb"
   "youtube_transcript"
   "time"
   "sequentialthinking"
   "semgrep"
   "resend"
   "papersearch"
   "openweather"
   "openapi-schema"
   "openapi"
   "node-code-sandbox"
   "minecraft-wiki"
   "microsoft-learn"
   "memory"
   "mcp-hackernews"
   "maven-tools-mcp"
   "markitdown"
   "gemini-api-docs"
   "filesystem"
   "everart"
   "stripe"
   "elevenlabs"])

(def fetch (memoize (fn [url] (try (:body (curl/get url)) (catch Throwable _ "")))))

(defn filter-names [coll] (->> coll (map :name)))

(defn read-catalog []
  (->> (slurp "/Users/slim/.docker/mcp/catalogs/docker-mcp.yaml")
       (yaml/parse-string)
       :registry
       (map (fn [[k v]] (assoc (select-keys v [:title :description :type :readme :toolsUrl]) :name (name k))))
       #_(map (fn [m] (update m :readme fetch)))
       (map (fn [m] (update m :toolsUrl (comp filter-names (fn [s] (json/parse-string s keyword)) fetch))))
       (map #(assoc % :tokens ((comp (partial tolkien/count-tokens "text-embedding-3-small") json/generate-string) %)))))

(comment

  (async/<!! (vec-db/create-collection vec-db-connection "hello2"))
  (async/<!! (vec-db/list-collections vec-db-connection))

  (count servers)
  (reduce +
          (for [s servers]
            (count
             (vals
              (json/parse-string (slurp (format "/Users/slim/docker/mcp-gateway/examples/tool_registrations/tool-json/%s.json" s)))))))
  (float (/ (reduce + (for [s servers]
                        (count (slurp (format "/Users/slim/docker/mcp-gateway/examples/tool_registrations/tool-json/%s.json" s))))) 4))

  ;; cleanup
  (async/<!!
   (async/go
     (doseq [item (async/<! (vec-db/list-collections vec-db-connection))]
       (println "delete " item)
       (async/<! (vec-db/delete-collection vec-db-connection (:name item))))))

  ;; make sure the model has been pulled
  (dmr-models)
  (dmr-get-model "ai" "embeddinggemma:latest")
  (dmr-create-model embedding-model)
  (dmr-create-model summary-model)

  ;; sembeds about 1 tool / second
  ;; embed using GPT text-embedding-3-small (dimension is 1536)
  ;; average 400ms per tool at 2m21s total
  (time
   (doseq [s servers]
     (async/<!!
      (embed-server-tools
       {:embedding-fn (partial dmr-embeddings "ai/qwen3-embedding") ;gpt-embeddings
        :summarize-fn json-with-token-check}
       {:name s}
       (vals
        (json/parse-string
         (slurp
          (format "/Users/slim/docker/mcp-gateway/examples/tool_registrations/tool-json/%s.json" s)) keyword))))))

  ;; embed servers
  (def catalog (read-catalog))
  (->> catalog
       (filter #(< 8191 (:tokens %)))
       (map #(select-keys % [:name :tokens])))
  (time
   (async/<!!
    (embed-servers
     {:embedding-fn (partial dmr-embeddings "ai/qwen3-embedding")  ;gpt-embeddings
      :summarize-fn json/generate-string}
     {:name "mcp-server-collection"}
     catalog)))

  ;; search tools
  (def search-config {:embedding-fn (partial dmr-embeddings "ai/qwen3-embedding") ;gpt-embeddings
                      :exclude_collections ["mcp-server-collection"]})
  (async/<!! (search search-config "I need to find github pull requests and I don't care about issues"))
  (async/<!! (search search-config "create a new pull request on github"))
  (async/<!! (search search-config "run bash on something"))
  (async/<!! (search search-config "do a wikipedia search"))
  (async/<!! (search search-config "are there any air bnb apartments in SF"))
  (async/<!! (search search-config "I need to do a security scan"))

  ;; search servers
  (def server-search-config {:collection_name "mcp-server-collection"
                             :embedding-fn (partial dmr-embeddings "ai/qwen3-embedding")})
  (async/<!! (search server-search-config "what if I need to integrate with different chat systems")))

(comment

  ; semgrep_scan 10781 bigger than 4096.  
  (doseq [s servers]
    (println
     s
     " -> "
     (->
      (vals (json/parse-string (slurp (format "/Users/slim/docker/mcp-gateway/examples/tool_registrations/tool-json/%s.json" s)) keyword))
      (json/generate-string)
      (count))))

  ; experiment - summarize all of the tool metadata
  (doseq [s servers]
    (summarize-tools
     (vals (json/parse-string (slurp (format "/Users/slim/docker/mcp-gateway/examples/tool_registrations/tool-json/%s.json" s)) keyword))))

  ;; all tools should have less than 2048 tokens in the data being embedded - should be empty 
  (->>
   (for [s servers]
     (for [tool (vals (json/parse-string (slurp (format "/Users/slim/docker/mcp-gateway/examples/tool_registrations/tool-json/%s.json" s)) keyword))]
       [s (-> tool :tool :name) (tolkien/count-tokens "text-embedding-3-small" (json-with-token-check tool))]))
   (apply concat)
   (filter (fn [[_ _ n]] (< 2048 n)))))

