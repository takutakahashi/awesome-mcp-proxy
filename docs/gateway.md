# MCP Server Gateway Specification

## 概要

MCP Server Gatewayは、複数のMCPサーバーへの統一されたアクセスポイントを提供する機能です。単一の `/mcp` エンドポイントを通じて、異なるバックエンドMCPサーバーが提供するツール、リソース、プロンプトに透過的にアクセスできるようにします。

## 背景

現在のawesome-mcp-proxyは、個別のMCPサーバーへのプロキシとして動作していますが、複数のMCPサーバーを統合して単一のエンドポイントから利用できる機能が必要です。これにより、クライアントは複数のバックエンドサーバーの存在を意識することなく、すべての機能にアクセスできるようになります。

## 機能要件

### 1. 統一エンドポイント
- **単一のエンドポイント**: `/mcp` を通じてすべてのバックエンドMCPサーバーにアクセス
- **透過的なルーティング**: クライアントはバックエンドサーバーの存在を意識しない
- **自動的な振り分け**: リクエスト内容に基づいて適切なバックエンドサーバーを自動選択

### 2. バックエンド管理
- **複数のトランスポート方式をサポート**:
  - HTTP/HTTPS
  - stdio (標準入出力)
  - WebSocket（将来的な拡張）
- **動的なバックエンド登録**: 設定ファイルベースでのバックエンド管理
- **ヘルスチェック**: バックエンドサーバーの可用性監視

### 3. 能力ディスカバリー
- **起動時の能力取得**: 各バックエンドサーバーの提供する機能を自動的に取得
  - ツール一覧 (`tools/list`)
  - リソース一覧 (`resources/list`)
  - プロンプト一覧 (`prompts/list`)
- **定期的な更新**: バックエンドの能力を定期的に再取得
- **統合ルーティングテーブル**: すべてのバックエンドの能力を統合管理

### 4. リクエストルーティング
- **メソッドベースのルーティング**:
  - `tools/call`: ツール名に基づいてバックエンドを選択
  - `resources/read`: URIパターンに基づいてバックエンドを選択
  - `prompts/get`: プロンプト名に基づいてバックエンドを選択
- **リスト系メソッドの集約**:
  - `tools/list`: 全バックエンドのツールを集約して返却
  - `resources/list`: 全バックエンドのリソースを集約して返却
  - `prompts/list`: 全バックエンドのプロンプトを集約して返却

### 5. エラーハンドリング
- **バックエンドエラーの処理**: 個別バックエンドの障害を適切に処理
- **フォールバック機構**: 可能な場合は代替バックエンドへのフォールバック
- **詳細なエラーレポート**: クライアントへの適切なエラー情報の提供

## 必須機能の仕様

### 動的Capability検出

MCP Server Gatewayは、**起動時に全バックエンドの能力を検出し、動的にcapabilityを決定**します。

#### Capability集約ルール

**すべてのcapabilityが動的に決定**されます：

1. **tools capability**: 
   - **1つでもバックエンドがサポートしていれば有効化**
   - サポートするバックエンドがない場合は省略

2. **resources capability**:
   - **1つでもバックエンドがサポートしていれば有効化**
   - サポートするバックエンドがない場合は省略

3. **prompts capability**:
   - **1つでもバックエンドがサポートしていれば有効化**
   - サポートするバックエンドがない場合は省略

#### 動的検出のアルゴリズム

```go
// Gateway起動時のcapability集約
func (g *Gateway) discoverCapabilities() GatewayCapabilities {
    capabilities := GatewayCapabilities{
        // すべて動的検出: バックエンドがサポートしていれば有効化
    }
    
    for _, group := range g.groups {
        for _, backend := range group.Backends {
            // 各バックエンドにinitializeを送信
            initResp, err := backend.Initialize()
            if err != nil {
                log.Printf("Backend %s initialization failed: %v", backend.Name, err)
                continue
            }
            
            // 返されたcapabilityを統合
            if initResp.Capabilities.Tools != nil {
                capabilities.Tools = true
            }
            if initResp.Capabilities.Resources != nil {
                capabilities.Resources = true
            }
            if initResp.Capabilities.Prompts != nil {
                capabilities.Prompts = true
            }
        }
    }
    
    return capabilities
}
```

#### initializeレスポンス例

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "protocolVersion": "2024-11-05",
    "capabilities": {
      "tools": {},     // バックエンドが1つでもサポートしていれば有効
      "resources": {}, // バックエンドが1つでもサポートしていれば有効  
      "prompts": {}    // バックエンドが1つでもサポートしていれば有効
    },
    "serverInfo": {
      "name": "mcp-gateway",
      "version": "1.0.0"
    }
  }
}
```

#### 動的メソッド

**必須メソッド**（常に実装）:
- `initialize` - 初期化と動的capability宣言

**動的メソッド**（バックエンドの能力に応じて有効化）:
- `tools/list`, `tools/call` - 1つでもバックエンドがtoolsをサポートする場合
- `resources/list`, `resources/read` - 1つでもバックエンドがresourcesをサポートする場合
- `prompts/list`, `prompts/get` - 1つでもバックエンドがpromptsをサポートする場合

### バックエンド不在時の挙動

バックエンドが一つも利用できない場合、Gatewayは以下の応答を返します：

1. **`initialize`**: capability無しの正常なレスポンス `{"capabilities": {}}`
2. **すべてのメソッド**: `{"error": {"code": -32601, "message": "Method not found"}}`

バックエンドの能力に応じてcapabilityとメソッドが動的に決定されます。

## 技術仕様

### 設定ファイル構造

```yaml
gateway:
  host: "0.0.0.0"
  port: 8080
  endpoint: "/mcp"
  timeout: 30s

groups:
  - name: "developer"
    backends:
      git-tools:
        name: "git-tools"
        transport: "stdio"
        command: "mcp-server-git"
        args: ["--repo", "/workspace"]
        env:
          GITHUB_TOKEN: "${GITHUB_TOKEN}"
          
      filesystem-tools:
        name: "filesystem-tools"
        transport: "http"
        endpoint: "http://localhost:3001/mcp"
        headers:
          Authorization: "Bearer ${FILESYSTEM_TOKEN}"
          
      docker-tools:
        name: "docker-tools"
        transport: "stdio"
        command: "mcp-docker"
        env:
          DOCKER_HOST: "${DOCKER_HOST}"

  - name: "designer"
    backends:
      figma-tools:
        name: "figma-tools"
        transport: "http"
        endpoint: "http://figma-mcp:3002/mcp"
        headers:
          X-Figma-Token: "${FIGMA_TOKEN}"
          
      asset-management:
        name: "asset-management"
        transport: "http"
        endpoint: "http://assets-mcp:3003/mcp"
        headers:
          Authorization: "Bearer ${ASSETS_TOKEN}"

middleware:
  logging:
    enabled: true
    level: "info"
    
  cors:
    enabled: true
    allowed_origins: ["*"]
    
  caching:
    enabled: true
    ttl: 300s
```

### ルーティングテーブル構造

```go
type RoutingTable struct {
    // ツール名 -> バックエンド名のマッピング
    ToolsMap     map[string]string
    
    // リソースURIパターン -> バックエンド名のマッピング
    ResourcesMap map[string]string
    
    // プロンプト名 -> バックエンド名のマッピング
    PromptsMap   map[string]string
    
    // バックエンド名 -> Backend実装のマッピング
    Backends     map[string]Backend
}
```

### リクエストフロー

```
1. クライアントリクエスト受信 (/mcp)
2. JSON-RPCメソッド解析
3. ルーティング判定:
   - tools/call -> ToolsMapを参照
   - resources/read -> ResourcesMapを参照
   - prompts/get -> PromptsMapを参照
   - list系 -> 全バックエンドから集約
4. バックエンドへのリクエスト転送
5. レスポンスの返却
```

## シーケンス図

### 1. 初期化シーケンス（起動時の能力ディスカバリー）

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

### 2. ツール実行のシーケンス (tools/call)

```mermaid
sequenceDiagram
    participant C as Client
    participant GW as Gateway
    participant RT as RoutingTable
    participant B as Backend<br/>(figma-tools)

    C->>GW: POST /mcp<br/>{method: "tools/call",<br/>params: {name: "figma_export"}}
    
    GW->>GW: Parse JSON-RPC
    
    GW->>RT: Lookup tool "figma_export"
    RT-->>GW: backend: "figma-tools"
    
    GW->>B: Forward request<br/>{method: "tools/call",<br/>params: {name: "figma_export"}}
    
    B-->>GW: Response<br/>{result: {...}}
    
    GW-->>C: JSON-RPC Response<br/>{result: {...}}
```

### 3. ツール一覧取得のシーケンス (tools/list) - 集約パターン

```mermaid
sequenceDiagram
    participant C as Client
    participant GW as Gateway
    participant Cache as Cache
    participant B1 as Backend1<br/>(git-tools)
    participant B2 as Backend2<br/>(filesystem)
    participant B3 as Backend3<br/>(figma-tools)

    C->>GW: POST /mcp<br/>{method: "tools/list"}
    
    GW->>Cache: Check cache
    
    alt Cache Hit
        Cache-->>GW: Cached tools list
        GW-->>C: JSON-RPC Response<br/>{result: {tools: [...]}}
    else Cache Miss
        GW->>GW: Aggregate from all backends
        
        par Parallel requests
            GW->>B1: tools/list
            B1-->>GW: [git_commit, git_status]
        and
            GW->>B2: tools/list
            B2-->>GW: [read_file, write_file]
        and
            GW->>B3: tools/list
            B3-->>GW: [figma_export, figma_import]
        end
        
        GW->>GW: Merge all tools
        
        GW->>Cache: Store in cache<br/>(TTL: 300s)
        
        GW-->>C: JSON-RPC Response<br/>{result: {tools: [<br/>  git_commit, git_status,<br/>  read_file, write_file,<br/>  figma_export, figma_import<br/>]}}
    end
```

### 実装の優先順位

#### Phase 1（最優先）
- ✅ **動的capability検出の完全実装**
  - `initialize` でバックエンドの能力に基づいてcapabilityを決定
  - すべてのメソッドを動的に有効化/無効化
  - バックエンドがない場合の適切なエラー処理

#### Phase 2
- 各capabilityの機能完全実装（tools, resources, prompts）
- 集約とルーティング機構の最適化

## 影響範囲

- **既存機能への影響**: 既存のプロキシ機能と並行して動作可能
- **互換性**: MCP仕様に完全準拠
- **移行パス**: 段階的な移行が可能

## リスクと対策

### リスク1: バックエンドサーバーの障害
- **対策**: ヘルスチェックと自動的な障害検出・隔離

### リスク2: パフォーマンス劣化
- **対策**: キャッシング機構とコネクションプーリング

### リスク3: セキュリティ
- **対策**: 適切な認証・認可機構の実装

## 参考資料

- [Model Context Protocol Specification](https://modelcontextprotocol.io/specification)
- [JSON-RPC 2.0 Specification](https://www.jsonrpc.org/specification)
- Issue #2: MCP Gateway/Proxy Configuration Management (Closed)
- Issue #8: Feature: MCP Server Gateway - Single Endpoint Proxy Implementation (Closed)
