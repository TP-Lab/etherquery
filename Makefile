TARGET=tb.etherquery.s
SOURCES=$(wildcard *.go)
VERSION ?= $(shell git describe --always --long --dirty)

$(TARGET): $(SOURCES) 
	go build -ldflags="-X main.version=${VERSION}" -o $@ $(SOURCES)

.PHONY: clean
clean:
	rm -f $(TARGET)

vendor: glide.yaml
	glide update
