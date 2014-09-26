APP_NAME=docker-builder

run: build
	./bin/${APP_NAME}

build: vet
	go build -v -o ./bin/${APP_NAME} ./src/${APP_NAME}.go

clean:
	rm -rf ./bin/*

lint:
	golint ./src

vet:
	go vet ./src

