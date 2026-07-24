package web

// この宣言に付与したコメントを swag が OpenAPI の一般情報として読み取る。
// 仕様の正は Go コード（このコメントと各ハンドラの注釈・DTO 型）であり、
// api/openapi.yaml は `make openapi`（swag）で自動生成する。手で編集しない。
//
//	@title			duo-pocketbook API
//	@version		1.0.0
//	@description	クライアント2人で使う家計簿アプリケーションのAPI。
//	@description	共有支出を2アカウントから登録し、月次で双方の収入を入力すると、指定した比重で双方の可処分所得が揃うよう精算額（振込額）を算出します。
//	@description	精算ロジック: 各メンバーの純額 net = 収入 - 立替済み共有支出 とし、比重 wA:wB に対して A→B の振込額 t = (wB*netA - wA*netB) / (wA + wB) を計算します（端数は四捨五入、負の場合はB→Aの振込）。精算後は 可処分所得A/wA == 可処分所得B/wB が成り立ちます。
//	@description	認証: `POST /login` で JWT を取得し、以降は `Authorization: Bearer <token>` を付与します（memberId にはログインIDを渡す。JWT の subject は不変の AccountID）。
//	@description	アクセス制限（任意）: バックエンドに CLIENT_KEY を設定して運用する場合、`/health` と CORS プリフライトを除く全リクエストに事前共有キー `X-Client-Key` ヘッダが必要です（不一致は 403 FORBIDDEN）。また `/login` は IP 単位のレート制限があり、超過時は 429 を返します。
//	@description	サーバー: ローカルは http://localhost:8080、本番は AWS Lambda Function URL。
//
//	@BasePath	/
//
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				JWT を `Authorization: Bearer <token>` 形式で送る。
const openapiGeneralInfo = "" // swag の -g 対象。上のコメントが一般情報として読まれる。
