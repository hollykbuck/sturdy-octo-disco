.PHONY: all

all:
	@echo "构建 build/main"
	go build -o build/main github.com/hollykbuck/honeydew/cmd

clean:
	rm -rf build/