build:
	go build -o bin/surf

run:
	./bin/surf

test:
	go test -v ./...

clean:
	rm -rf bin
