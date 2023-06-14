del -Recurse -Force .\build\*
gofmt -e -s -w .
go mod tidy
$LDFLAGS="-ldflags=-X 'main.buildTime=$(date)' -X 'main.lastCommit=$(git rev-parse HEAD)' -X 'main.semanticVersion=$(git describe --tags --dirty=-dev)'"
go build -o build/ ${LDFLAGS} ./cmd/celestia
go build -o build/ ./cmd/cel-key