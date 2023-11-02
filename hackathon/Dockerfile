FROM golang:1.18 as build
WORKDIR /
# ホストOSのカレントディレクトリにあるすべてのファイルをコンテナ内にコピー
COPY . /
# コンテナの起動時に実行するコマンドを指定
CMD ["go", "run", "main.go"]

#docker run --publish 8000:8000 <image id> で起動すること
#参照:https://docs.docker.jp/language/nodejs/run-containers.html