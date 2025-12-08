---
theme: gaia
_class: lead
paginate: true
backgroundColor: #fff
backgroundImage: url('https://marp.app/assets/hero-background.svg')
---

<script type="module">
  import mermaid from 'https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs';
  mermaid.initialize({ startOnLoad: true });
</script>

![bg left:40% 80%](assets/docker-mark-blue.svg)

# **Dynamic Tools**

**mcp-find**: Tool Embeddings

https://github.com/docker/mcp-gateway

---

# Scenario

* 41 _active_ servers
*  => 335 tools
*  => 209K tokens of tool description / request
*  => need to be careful at $1.25/1M tokens

---

# Tool Broker

<div class="mermaid">
  sequenceDiagram
    Agent->>Gateway: tools/list
    Gateway-->>Agent: [empty]
    Agent->>Gateway: mcp-find(context)
    Gateway-->>Agent: [tools]
    Agent->>Gateway: mcp-exec
</div>

---

# Improve **mcp-find**

* currently using a keyword search on an in-memory index

---

#### Dynamic Embeddings

<div class="mermaid">
  flowchart LR
    VectorStore["`**VectorStore** (sqlite-vec)`"]
    Gateway-->VectorStore
    Gateway-->EmbeddingModel
    Gateway-->SummaryModel
</div>

* generate embeddings on the fly
* tool definitions are not always static

---

# Dynamic Embeddings

| Model                  | time(ms)/tool  | dim   | ctx len | size  |  Notes  |
| :---                             | :--  | :---  | :---    | :---  | :---- | 
| DMR - embeddinggemma (302M)      | 4871 | 768   | 2048    | 307M  | needs tool summarization | 
| DMR - qwen3-embedding (4B)       | 920  | 2560  | 40960   | 2.32G | can embed un-summarized tool def | 
| GPT (text-embedding-3-small)     | 307  | 1536  | 8191    |  -    | can embed un-summarized tool def |
| DMR - nomic (137M)               |      | 768   | 2048    | 0.5G  | needs tool summarization | 

* 40 servers will be about 4 megs for larger vec dimensions like qwen3.  Roughly half that for text-embedding-3-small, and half again for the smaller dimensions.

Pre-summary to use smaller models

* let's look at gemma3 first.
   * can't be used for embeddings - can summarize 4096 context but still too small
* 

---

# Current: mcp-find/mcp-exec

<div class="mermaid">
  sequenceDiagram
    Agent->>Gateway: tools/list
    Gateway-->>Agent: [empty]
    Agent->>Gateway: mcp-find(context)
    Gateway-->>Agent: [tools]
    Agent->>Gateway: mcp-exec
</div>

---

# Custom Agent Loop

<div class="mermaid">
  sequenceDiagram
    Agent->>Gateway: tools/list
    Gateway-->>Agent: [empty]
    Agent->>Gateway: mcp-find(context)
    Agent-->>Agent: update-tool-list
    Agent->>Gateway: tools/call
</div>

---

# Next Steps

1. compare `mcp-find/mcp-exec` with `custom agent loop` => blog
    * community engagement: **mcp-exec** is weird 
2. explore distributing static embeddings
    * just for our catalog?

---

# Marp MCP summary

This slide deck was authored from [8f63ff759892d9b1d591e03e3d2e2dcbe1387012](https://github.com/docker/mcp-gateway/commits/main/).
