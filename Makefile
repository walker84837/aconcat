GO_BUILD = go build -ldflags="-s -w" -o
BINARY_NAME = ac
SRC_DIR = src
OUTPUT_DIR = bin
SRC_FILES = $(wildcard src/*.go)

# Default target
all: build

# Build target
build: $(SRC_FILES)
	@mkdir -p $(OUTPUT_DIR)
	$(GO_BUILD) $(OUTPUT_DIR)/$(BINARY_NAME) $(SRC_FILES)

# Clean target
clean:
	rm -r $(OUTPUT_DIR)

# Run the program
run: build
	$(OUTPUT_DIR)/$(BINARY_NAME)

# Test target (if you have tests)
test:
	go test ./...

.PHONY: all build clean run test
