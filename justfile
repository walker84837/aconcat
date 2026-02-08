GO_BUILD := "go build -trimpath -buildmode=pie -mod=readonly -modcacherw -ldflags=\"-s -w\" -o"
BINARY_NAME := "ac"
SRC_DIR := "src"
OUTPUT_DIR := "bin"
SRC_FILES := `ls src/*.go 2>/dev/null || true`

default: build

build:
  mkdir -p {{OUTPUT_DIR}}
  {{GO_BUILD}} {{OUTPUT_DIR}}/{{BINARY_NAME}} {{SRC_FILES}}

clean:
  rm -rf {{OUTPUT_DIR}}

run: build
  {{OUTPUT_DIR}}/{{BINARY_NAME}}

test:
  go test ./...
