# 開発メモ

## 境界

共通スキーマと入出力処理は当面各リポジトリ内に配置。独立ビルドを優先し、共有ライブラリの抽出は後で行います。
ライブ取得は client-go ではなくインストール済み kubectl の get のみを実行します。

## 必要な RBAC

対象リソースの get/list が必要です。watch/update/patch/create/delete は不要です。

対象 Namespace: workloads, localqueues, pods。クラスタスコープ: clusterqueues, resourceflavors。

## 検証

`make check` で vet と race test。`make build` でローカルバイナリ。
APIアクセス、実通信、実GPUはfixtureテストとは別の検証です。
