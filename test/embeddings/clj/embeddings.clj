(ns embeddings
  (:require
   [babashka.curl :as curl]
   [cheshire.core :as json]
   [clj-yaml.core :as yaml]
   [clojure.core.async :as async]
   [tolkien.core :as tolkien]
   [vector-db-process :as vec-db]
   [dmr]))

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

(defn embed-servers
  "embed the server descriptions"
  [{:keys [embedding-fn summarize-fn connection]} collection servers]
  (println "> embed collection" (:name collection))
  (async/go
    (async/<! (vec-db/delete-collection connection (:name collection)))
    (async/<! (vec-db/create-collection connection (:name collection)))
    (doseq [server servers
            :let [summary (time (summarize-fn server))]]
      (let [vec (time (dmr/create-embedding embedding-fn summary))]
        (println "  > embed server" (-> server :name) " -> " (count summary))
        (async/<!! (vec-db/add-vector connection (:name collection) vec (select-keys server [:name])))))))

(defn embed-server-tools
  "embed the server descriptions"
  [{:keys [embedding-fn summarize-fn connection]} collection tool-registrations]
  (println "> embed collection" (:name collection))
  (async/go
    (async/<! (vec-db/delete-collection connection (:name collection)))
    (async/<! (vec-db/create-collection connection (:name collection)))
    (doseq [tool-registration tool-registrations
            :let [summary (time (summarize-fn tool-registration))]]
      (let [vec (time (dmr/create-embedding embedding-fn summary))]
        (println "  > embed tool" (-> tool-registration :tool :name) " -> " (count summary))
        (async/<!! (vec-db/add-vector connection (:name collection) vec {:tool (select-keys (:tool tool-registration) [:name])}))))))

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

(defn cleanup-vectors [{:keys [connection]}]
  (async/go
    (doseq [item (async/<! (vec-db/list-collections connection))]
      (println "delete " item)
      (async/<! (vec-db/delete-collection connection (:name item))))))

(comment
  (require '[clojure.edn :as edn])

  (count servers)
  (reduce +
          (for [s servers]
            (count
             (vals
              (json/parse-string (slurp (format "/Users/slim/docker/mcp-gateway/examples/tool_registrations/tool-json/%s.json" s)))))))
  (float (/ (reduce + (for [s servers]
                        (count (slurp (format "/Users/slim/docker/mcp-gateway/examples/tool_registrations/tool-json/%s.json" s))))) 4))

  ;; cleanup
  (async/<!! (cleanup-vectors qwen3-config))

  ;; make sure the model has been pulled
  (dmr/dmr-models)
  (dmr/dmr-get-model "ai" "embeddinggemma:latest")
  (dmr/dmr-create-model "ai/embeddinggemma:latest")
  (dmr/dmr-create-model "ai/gemma3-qat:latest")

  (def qwen3-config
    {:embedding-fn (partial dmr/dmr-embeddings "ai/qwen3-embedding")  ;gpt-embeddings
     :summarize-fn json-with-token-check
     :connection (vec-db/vector-db-stdio-server {:name "qwen3-vectors" :dimension 2560 :db "qwen3-vectors.db"})})

  (def gpt-config
    {:embedding-fn dmr/gpt-embeddings
     :summarize-fn json-with-token-check
     :connection (vec-db/vector-db-stdio-server {:name "gpt-vectors" :dimension 1536 :db "gpt-vectors.db"})})

  (def gemma-config
    {:embedding-fn (partial #'dmr/dmr-embeddings "ai/embeddinggemma:latest")
     :summarize-fn (comp (partial #'dmr/summarize-tool (partial #'dmr/dmr-completion "ai/gemma3-qat:latest")) json/generate-string)
     :connection (vec-db/vector-db-stdio-server {:name "gemma-vectors" :dimension 768 :db "gemma-vectors.db"})})

  ;; sembeds about 1 tool / second
  ;; embed using GPT text-embedding-3-small (dimension is 1536)
  ;; average 400ms per tool at 2m21s total
  (doseq [config [qwen3-config gpt-config gemma-config]]
    (time
     (doseq [s servers]
       (async/<!!
        (embed-server-tools
         config
         {:name s}
         (vals
          (json/parse-string
           (slurp
            (format "/Users/slim/docker/mcp-gateway/examples/tool_registrations/tool-json/%s.json" s)) keyword)))))))

  ;; embed servers
  (def catalog (read-catalog))
  (spit "full-catalog.edn" (pr-str catalog))
  (def catalog (edn/read-string (slurp "full-catalog.edn")))
  (->> catalog
       (filter #(< 8191 (:tokens %)))
       (map #(select-keys % [:name :tokens])))
  (time
   (async/<!!
    (embed-servers
     gemma-config
     {:name "mcp-server-collection"}
     catalog)))

  ;; search tools
  (def search-config (merge gemma-config
                            {:exclude_collections ["mcp-server-collection"]}))
  (async/<!! (dmr/search search-config "I need to find github pull requests and I don't care about issues"))
  (async/<!! (dmr/search search-config "create a new pull request on github"))
  (async/<!! (dmr/search search-config "run bash on something"))
  (async/<!! (dmr/search search-config "do a wikipedia search"))
  (async/<!! (dmr/search search-config "are there any air bnb apartments in SF"))
  (async/<!! (dmr/search search-config "I need to do a security scan"))
  (async/<!! (dmr/search search-config "I need to search slack channels"))

  ;; search servers
  (def server-search-config (merge gpt-config {:collection_name "mcp-server-collection"}))
  (async/<!! (dmr/search server-search-config "I need to search slack channels")))

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

  ;; all tools should have less than 2048 tokens in the data being embedded - should be empty 
  (->>
   (for [s servers]
     (for [tool (vals (json/parse-string (slurp (format "/Users/slim/docker/mcp-gateway/examples/tool_registrations/tool-json/%s.json" s)) keyword))]
       [s (-> tool :tool :name) (tolkien/count-tokens "text-embedding-3-small" (json-with-token-check tool))]))
   (apply concat)
   (filter (fn [[_ _ n]] (< 2048 n)))))


