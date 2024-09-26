clean:
	rm -rf output

build: clean
	go build -o output/vbuilder .

install: build
	cp -f output/* /home/vorticist/.local/bin