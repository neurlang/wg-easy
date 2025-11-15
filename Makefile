.PHONY: build run clean install

build:
	go build -o wg-easy-go

run: build
	sudo ./wg-easy-go

clean:
	rm -f wg-easy-go

install:
	go mod download

docker-build:
	docker build -t wg-easy-go .

docker-run:
	docker run --rm -it \
		--cap-add=NET_ADMIN \
		--cap-add=SYS_MODULE \
		-v /lib/modules:/lib/modules \
		-v $(PWD)/config.json:/app/config.json \
		-p 8080:8080 \
		-p 51820:51820/udp \
		wg-easy-go
