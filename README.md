# OBS Studio　コード証明書問題回避用プログラム

OBS Studioのwin-captureプラグインとフックDLLを簡単に更新するためのユーティリティです。  
このツールは、OBSの画面キャプチャコンポーネントを更新する際の手動作業を自動化します。

# 注意
100%の動作は保証しません。  
OBSのバックアップは念のためにとっておきましょう。  
※大体は下記のパスにあります。OBSを終了してzipファイルにでも固める。  
C:\Users\<使用しているPCのユーザ名>\AppData\Roaming\obs-studio  

何か不具合あったら [ @ureshinovt](https://x.com/ureshinovt)  

# ざっくりやってること
OBSに組み込まれているプラグインである、win-capture を VALORANT(アンチチート VANGUARD)で対応されているバージョンにダウングレードします。

## 機能

- ProgramData\obs-studio-hookの既存DLLファイルを自動的にバックアップ
- ZIPファイルから新しいwin-captureファイルを抽出して適切な場所に配置
- Windows標準のファイル選択ダイアログでZIPファイルを選択可能
- 管理者権限の自動チェックと昇格
- OBSのインストール場所を自動検出

## 動作要件

- Windows 10/11
- OBS Studioがインストールされていること

## インストール方法

1. 最新のリリースから`obs_wincap_patch.exe`をダウンロード
2. 下記のzipファイルをOBS公式Githubよりダウンロード  
https://github.com/obsproject/obs-studio/releases/download/30.1.2/OBS-Studio-30.1.2.zip

3. 1・2でダウンロードしたファイルを、任意の場所に保存するだけで使用可能（インストールは不要）  
※特定の権限が不足しているフォルダ以下では試していないため、ダウンロードしたフォルダで実行したほうが早いかも


## 使用方法

1. `obs_wincap_patch.exe`を実行します（管理者権限は自動的に要求されます）
2. 表示されるファイル選択ダイアログで、更新用のZIPファイルを選択します
3. プログラムが以下の処理を自動的に実行します：
   - ProgramData\obs-studio-hookフォルダ内のDLLファイルを「old」フォルダに移動
   - 選択したZIPファイルからwin-captureフォルダを抽出
   - OBSインストール先のwin-captureフォルダにファイルをコピー
4. 処理が完了したら、OBS Studioを再起動して更新を適用します

## 仕組み

このツールは以下の処理を実行します：

1. **管理者権限の確認**：
   - プログラムが管理者権限で実行されているか確認
   - 権限がない場合は自動的に管理者権限で再起動

2. **パスの検出**：
   - `%ProgramData%\obs-studio-hook`フォルダを特定
   - レジストリまたは一般的なインストール先からOBSのインストール場所を検出
   - `obs-studio\data\obs-plugins\win-capture`フォルダを特定

3. **バックアップ**：
   - `%ProgramData%\obs-studio-hook`内のすべてのDLLファイルを`old`サブフォルダに移動

4. **更新**：
   - 選択したZIPファイルを一時フォルダに解凍
   - win-captureフォルダを検索
   - ファイルをOBSインストール先の適切な場所にコピー

### ビルドするには

Goの開発環境がインストールされている場合：

```bash
git clone https://github.com/ureshinosan-undo/obs-patch-wincapture.git
cd obs-patch-wincapture
go mod init obs_wincap_patch
go build -o obs_wincap_patch.exe
```

## トラブルシューティング

### エラー: OBSのインストール先が見つかりません
- OBSが標準的な場所にインストールされていない可能性があります。
- 今後のバージョンでは手動選択をサポート予定です。

### エラー: ZIPファイル内にwin-captureフォルダが見つかりません
- 更新用ZIPファイルの構造が正しくない可能性があります。
- 正しいフォルダ構造を持つZIPファイルを使用してください。

### OBSが起動しない、または機能が動作しない
- バックアップされたDLLファイルを復元するには：
  1. `%ProgramData%\obs-studio-hook\old`フォルダ内のDLLファイルを
  2. `%ProgramData%\obs-studio-hook`フォルダに戻してください。

## 免責事項

このツールは自己責任でご利用ください。OBS Studioの公式ツールではありません。更新前に重要なOBS設定のバックアップを取ることをお勧めします。
