(ns officialregistry
  (:require [cheshire.core :as json]
            [clj-http.lite.client :as http]))

(def registry-url "https://registry.modelcontextprotocol.io/v0/servers")

(defn fetch-page
  "Fetch a single page of servers from the MCP registry.
  Returns a map with :servers and :next-cursor keys."
  ([] (fetch-page nil))
  ([cursor]
   (let [url (if cursor
               (str registry-url "?cursor=" cursor)
               registry-url)
         response (http/get url)
         data (json/parse-string (:body response) true)]
     {:servers (get data :servers [])
      :next-cursor (get-in data [:metadata :next_cursor])})))

(defn page-all-servers
  "Lazily page through all servers in the registry.
  Returns a lazy sequence of server maps."
  ([] (page-all-servers nil))
  ([cursor]
   (lazy-seq
     (let [{:keys [servers next-cursor]} (fetch-page cursor)]
       (if (seq servers)
         (concat servers
                 (when next-cursor
                   (page-all-servers next-cursor)))
         [])))))

(defn collect-all-servers
  "Eagerly collect all servers from the registry into a vector."
  []
  (vec (page-all-servers)))

(defn has-oci-package? [m]
  (some #(= "oci" (:registry_type %)) (:packages m)))

(def servers (collect-all-servers))
(count servers)
(->> servers
     (filter has-oci-package?)
     (map :name)
     (count))

(defn page-servers-with-callback
  "Page through servers, calling callback-fn with each page of results.
  callback-fn receives a vector of servers for each page."
  [callback-fn]
  (loop [cursor nil]
    (let [{:keys [servers next-cursor]} (fetch-page cursor)]
      (when (seq servers)
        (callback-fn servers)
        (when next-cursor
          (recur next-cursor))))))
