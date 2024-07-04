build:
	go build -o bin/surf

run:
	./bin/surf

test:
	go test -v ./...

generate-config:
	curl --output wgcf -L "https://github.com/ViRb3/wgcf/releases/download/v2.2.22/wgcf_2.2.22_linux_amd64"
	chmod +x wgcf
	wgcf register
	wgcf generate
	rm wgcf
	echo -e "[Socks5]\nBindAddress = 0.0.0.0:25344" >> wgcf-profile.toml

clean:
	rm -rf bin
