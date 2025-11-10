# MCP Server Gateway シーケンス図

## 1. 初期化シーケンス（起動時の能力ディスカバリー）

```mermaid
sequenceDiagram
    participant GW as Gateway
    participant RT as RoutingTable
    participant B1 as Backend1<br/>(git-tools)
    participant B2 as Backend2<br/>(filesystem)
    participant B3 as Backend3<br/>(figma-tools)

    Note over GW: Gateway起動
    GW->>RT: Initialize RoutingTable
    
    par Capability Discovery
        GW->>B1: initialize
        B1-->>GW: {capabilities: {tools: {}, prompts: {}}}
        
        GW->>B1: tools/list
        B1-->>GW: [git_commit, git_status, ...]
        
        GW->>B1: prompts/list
        B1-->>GW: [code_review, ...]
    and
        GW->>B2: initialize
        B2-->>GW: {capabilities: {tools: {}, resources: {}}}
        
        GW->>B2: tools/list
        B2-->>GW: [read_file, write_file, ...]
        
        GW->>B2: resources/list
        B2-->>GW: [file://*, ...]
    and
        GW->>B3: initialize
        B3-->>GW: {capabilities: {tools: {}}}
        
        GW->>B3: tools/list
        B3-->>GW: [figma_export, ...]
    end
    
    Note over GW: 統合Capability決定:<br/>✅ tools (B1,B2,B3が対応)<br/>✅ resources (B2が対応)<br/>✅ prompts (B1が対応)
    
    GW->>RT: Register mappings<br/>(tools, resources, prompts)
    
    Note over RT: ルーティングテーブル構築完了<br/>toolsMap: {<br/>  "git_commit": "git-tools",<br/>  "read_file": "filesystem",<br/>  "figma_export": "figma-tools"<br/>}
```

## 2. ツール一覧取得のシーケンス (tools/list) - メタツール提供

```mermaid
sequenceDiagram
    participant C as Client
    participant GW as Gateway

    C->>GW: POST /mcp<br/>{method: "tools/list"}
    
    Note over GW: メタツールのみ提供<br/>（コンテキスト圧縮防止）
    
    GW-->>C: JSON-RPC Response<br/>{result: {tools: [<br/>  {name: "list_tools", ...},<br/>  {name: "describe_tool", ...},<br/>  {name: "call_tool", ...}<br/>]}}
```

## 3. メタツール使用のシーケンス（完全なワークフロー）

```mermaid
sequenceDiagram
    participant C as Client
    participant GW as Gateway
    participant RT as RoutingTable
    participant B1 as Backend1<br/>(git-tools)
    participant B2 as Backend2<br/>(filesystem)
    participant Cache as Cache

    Note over C: メタツール必須ワークフロー
    
    C->>GW: tools/call<br/>{name: "list_tools"}
    
    GW->>Cache: Check cache
    
    alt Cache Hit
        Cache-->>GW: Cached tools list
    else Cache Miss
        par Parallel requests
            GW->>B1: tools/list
            B1-->>GW: [git_commit, git_status]
        and
            GW->>B2: tools/list  
            B2-->>GW: [read_file, write_file]
        end
        
        GW->>GW: Aggregate tool names
        GW->>Cache: Store in cache<br/>(TTL: 300s)
    end
    
    GW-->>C: ["git_commit", "read_file", ...]
    
    C->>GW: tools/call<br/>{name: "describe_tool",<br/>arguments: {"tool_name": "git_commit"}}
    GW->>RT: Lookup "git_commit"
    RT-->>GW: backend: "git-tools"
    GW->>B1: tools/list (get full definition)
    B1-->>GW: tool definition
    GW-->>C: tool description & schema
    
    C->>GW: tools/call<br/>{name: "call_tool",<br/>arguments: {<br/>  "tool_name": "git_commit",<br/>  "arguments": {"message": "fix bug"}<br/>}}
    GW->>RT: Lookup "git_commit"
    RT-->>GW: backend: "git-tools"
    GW->>B1: tools/call<br/>{name: "git_commit",<br/>arguments: {"message": "fix bug"}}
    B1-->>GW: execution result
    GW-->>C: tool execution result
```

## 4. 直接ツール呼び出し拒否のシーケンス

```mermaid
sequenceDiagram
    participant C as Client
    participant GW as Gateway

    C->>GW: POST /mcp<br/>{method: "tools/call",<br/>params: {name: "git_commit", ...}}
    
    Note over GW: 直接ツール呼び出し検出<br/>❌ 禁止されたアクセスパターン
    
    GW-->>C: JSON-RPC Error<br/>{error: {<br/>  code: -32601,<br/>  message: "Direct tool access forbidden. Use meta-tools: call_tool"<br/>}}
    
    Note over C: メタツール経由で再試行が必要
```

## 5. リソース読み取りのシーケンス (resources/read)

```mermaid
sequenceDiagram
    participant C as Client
    participant GW as Gateway
    participant RT as RoutingTable
    participant B as Backend<br/>(filesystem)

    C->>GW: POST /mcp<br/>{method: "resources/read",<br/>params: {uri: "file:///workspace/test.txt"}}
    
    GW->>GW: Parse JSON-RPC
    
    GW->>RT: Match URI pattern<br/>"file:///workspace/test.txt"
    RT-->>GW: backend: "filesystem"
    
    GW->>B: Forward request<br/>{method: "resources/read",<br/>params: {uri: "file:///workspace/test.txt"}}
    
    B-->>GW: Response<br/>{result: {contents: "..."}}
    
    GW-->>C: JSON-RPC Response<br/>{result: {contents: "..."}}
```

## 6. エラーハンドリングのシーケンス

```mermaid
sequenceDiagram
    participant C as Client
    participant GW as Gateway
    participant RT as RoutingTable
    participant B1 as Backend1<br/>(Primary)
    participant B2 as Backend2<br/>(Fallback)

    C->>GW: POST /mcp<br/>{method: "tools/call",<br/>params: {name: "call_tool", arguments: {...}}}
    
    GW->>RT: Lookup tool in arguments
    RT-->>GW: backend: "backend1"
    
    GW->>B1: Forward request
    
    alt Backend Error
        B1--xGW: Connection timeout
        
        Note over GW: Error detected
        
        GW->>RT: Find fallback backend
        RT-->>GW: backend: "backend2"
        
        GW->>B2: Forward request<br/>(Retry with fallback)
        B2-->>GW: Response<br/>{result: {...}}
        
        GW-->>C: JSON-RPC Response<br/>{result: {...}}
    else Success
        B1-->>GW: Response<br/>{result: {...}}
        GW-->>C: JSON-RPC Response<br/>{result: {...}}
    end
```

## 7. 並行バックエンド初期化シーケンス

```mermaid
sequenceDiagram
    participant GW as Gateway
    participant G1 as Group1<br/>(developer)
    participant G2 as Group2<br/>(designer)
    participant G3 as Group3<br/>(director)

    Note over GW: 設定ファイル読み込み
    
    par Parallel Group Initialization
        GW->>G1: Initialize developer group
        loop Each backend in group
            G1->>G1: Initialize git-tools
            G1->>G1: Initialize filesystem
            G1->>G1: Initialize docker-tools
        end
        G1-->>GW: Group ready
    and
        GW->>G2: Initialize designer group
        loop Each backend in group
            G2->>G2: Initialize figma-tools
            G2->>G2: Initialize asset-management
        end
        G2-->>GW: Group ready
    and
        GW->>G3: Initialize director group
        loop Each backend in group
            G3->>G3: Initialize project-management
            G3->>G3: Initialize analytics-tools
        end
        G3-->>GW: Group ready
    end
    
    Note over GW: All backends initialized<br/>Gateway ready to serve
```

## 8. 動的バックエンド更新シーケンス

```mermaid
sequenceDiagram
    participant Admin as Admin
    participant GW as Gateway
    participant RT as RoutingTable
    participant NewB as New Backend
    participant Cache as Cache

    Admin->>GW: Update config<br/>(Add new backend)
    
    GW->>GW: Reload configuration
    
    GW->>NewB: Initialize
    NewB-->>GW: capabilities
    
    par Discover capabilities
        GW->>NewB: tools/list
        NewB-->>GW: [new_tool1, new_tool2]
    and
        GW->>NewB: resources/list
        NewB-->>GW: [custom:///*]
    and
        GW->>NewB: prompts/list
        NewB-->>GW: [custom_prompt]
    end
    
    GW->>RT: Update routing table
    
    GW->>Cache: Invalidate cache
    
    Note over GW: New backend ready<br/>No downtime
```
