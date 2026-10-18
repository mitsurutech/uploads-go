# Go でファイルアップロード：バイト列は S3、それ以外は Postgres

[English](README.md) | [Français](README.fr.md) | [日本語](README.ja.md) | [Español](README.es.md)

[MitsuruTech 学習プラットフォーム](https://mitsurutech.co.uk/ja)のサンプルアプリです。ファイルをアップロードすると、
S3 がそれを保管し、Postgres がその行き先と名前を覚えています。ページの表は Postgres から、
ダウンロードはすべて S3 から取り出されます。

あえて小さくしてあり、ルートは4つ、ファイルは3つです。手を加える前に、全体を読み通せます。

```
main.go    ウェブ部分：4つのルートだけ
store.go   2つのストアと、書き込む順番
page.go    HTML
```

## ここにデプロイする

プラットフォームで **Deploy** を開き、このリポジトリを選んで、**データベース**と
**クラウドスタック**を一緒に起動します。アプリに必要なものはすでに環境に入っているので、
設定することはありません。

| 変数 | 設定するもの | 内容 |
| --- | --- | --- |
| `DATABASE_URL` | セッション | `db` という名前で動く Postgres サーバー |
| `AWS_ENDPOINT_URL` | セッション | `cloud` という名前で動くクラウドスタック |
| `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` | セッション | クラウドスタックの認証情報 |
| `PORT` | プラットフォーム | 待ち受けるポート |
| `BUCKET_NAME` | 任意（自分で設定） | 既定は `uploads` |

プラットフォームは `go build -o app .` でビルドし、`./app` で起動します。

## 読んでおきたい3つのこと

**1. `store.go` の `o.UsePathStyle = true`。** 最初に誰もが見落とす設定です。既定では
AWS SDK はバケット名をホスト名に入れて `uploads.cloud` にアクセスしますが、セッションの
ネットワークにそのような名前はないため、どの呼び出しもバケットとは無関係に見える DNS
エラーで失敗します。パススタイルにすると、代わりに `http://cloud:4566/uploads/...` に
アクセスします。

**2. アプリが自分でバケットを作る。** クラウドスタックは、開設したばかりの本物の AWS
アカウントと同じように、完全に空の状態で始まります。`CreateBucket` はコードの仕事で、
起動のたびに実行され、「すでに所有しています」というエラーは受け流すので、再起動しても
落ちません。

**3. 書き込みは S3 が先、行はあと。削除は行が先、S3 はあと。**

- バイト列をアップロードしてから、行を挿入します。挿入に失敗しても、誰も指していない
  オブジェクトが残るだけです。見えず、害はなく、あとで掃除できます。
- 行を先に書くと、アップロードに失敗したときに、開けないファイルがページに残り、
  その行を正常な行と見分ける方法がありません。
- 削除は、同じ理由で逆の順番になります。

一方の失敗は、わずかな無駄なバイトです。もう一方は、壊れたページです。害なく孤立して
よいほうを、必ず先に通してください。

## サンプルでも本気で守る2つのセキュリティ習慣

**S3 のキーは uuid で、ファイル名は決して使わない。** 2人が `notes.txt` をアップロード
しても、一方がもう一方を上書きしてはいけません。またブラウザーから来た名前をもとに
パスを組み立ててはいけません（`../../etc/passwd` は、フォームにとっては完全に有効な
ファイル名です）。本当の名前は列に入れます。そこなら安全です。

**ダウンロードは常に `attachment`、常に `application/octet-stream`、常に `nosniff`。**
誰でも何でもアップロードできます。HTML ファイルをインラインで返すと、アプリ自身の
アドレスのページとして、そのページが届くものすべてに触れながら実行されてしまいます。
ディスクに保存するだけなら無害です。

また、ファイル名はページに戻すときに必ずエスケープします。`<img src=x onerror=alert(1)>`
というファイル名は有効なファイル名であり、マークアップになってはいけません。

## 自分のマシンで動かす

Go、Postgres、S3 のようなものが必要です。Docker を使う場合：

```bash
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=dev --name pg postgres:16
docker run -d -p 4566:4566 --name cloud localstack/localstack

export DATABASE_URL=postgres://postgres:dev@localhost:5432/postgres
export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1

go build -o app . && ./app
```

そのあと <http://localhost:8080> を開きます。

## 次に試してみること

- 大きなアップロードをメモリに載せず、S3 に直接ストリーミングする（`main.go` に場所が書いてあります）。
- ダウンロード回数を DynamoDB に記録する。クラウドスタックはこれもエミュレートします。
- ファイルをログインの向こうに置いて、表を人ごとのものにする。
- バイト列を中継せず、署名付き URL を使って画像のプレビューを表示する。

## 詳しくは

- [GitHub からアプリをデプロイする](https://mitsurutech.co.uk/ja/help/deploy-app)：
  プラットフォームがこのようなリポジトリをどう動かすか。
- [クラウドスタック](https://mitsurutech.co.uk/ja/help/cloud-stack)：アプリが使えるように、
  セッションが起動する S3 風のサービス。
- クラスを教えていますか？[最初の1年は無料](https://mitsurutech.co.uk/ja/pricing)で、
  生徒50人まで使えます。[サインアップ](https://mitsurutech.co.uk/ja/signup)に必要なのはメールアドレスだけです。

## この演習について

[Mitsuru Technologies Ltd](https://mitsurutech.co.uk/ja/learning)（イングランド・ウェールズで登録、会社番号 16755264）が作っています。同社は学校とカレッジのための
[MitsuruTech 学習プラットフォーム](https://mitsurutech.co.uk/ja/learning)を開発しています。

ほかの言語の同じサンプル：[Node](https://github.com/mitsurutech/uploads-node)、
[Python](https://github.com/mitsurutech/uploads-python)、
[Java](https://github.com/mitsurutech/uploads-java)。

## ライセンス

この作品は [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/) でライセンスされています。Mitsuru Technologies Ltd のクレジットを表示し、商用に使わず、これをもとに作ったものを同じライセンスで共有するなら、学習と授業のために共有・改変できます。全文は `LICENSE` ファイルにあります。
