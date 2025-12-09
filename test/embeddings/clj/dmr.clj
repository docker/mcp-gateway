(ns dmr
  (:require
   [babashka.curl :as curl]
   [cheshire.core :as json]
   [vector-db-process :as vec-db]))

;; ==================================================
;; DMR
;; ==================================================

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
(defn search [{:keys [embedding-fn connection] :as options} s]
  (let [vec (create-embedding embedding-fn s)]
    (vec-db/search-vectors connection vec options)))

