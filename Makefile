all: builds/goreceptor.linux.amd64 builds/goreceptor.linux.arm64 builds/goreceptor.darwin.amd64 builds/goreceptor.darwin.arm64 builds/gocomandar.linux.amd64 builds/gocomandar.linux.arm64 builds/gocomandar.darwin.amd64 builds/gocomandar.darwin.arm64

clean:
	rm -f builds/*

builds/goreceptor.linux.amd64: goreceptor.go goalarmeitbl/*.go
	( GOOS=linux GOARCH=amd64 go build -o $@ goreceptor.go )

builds/goreceptor.linux.arm64: goreceptor.go goalarmeitbl/*.go
	( GOOS=linux GOARCH=arm64 go build -o $@ goreceptor.go )

builds/goreceptor.darwin.amd64: goreceptor.go goalarmeitbl/*.go
	( GOOS=darwin GOARCH=amd64 go build -o $@ goreceptor.go )

builds/goreceptor.darwin.arm64: goreceptor.go goalarmeitbl/*.go
	( GOOS=darwin GOARCH=arm64 go build -o $@ goreceptor.go )

builds/gocomandar.linux.amd64: gocomandar.go goalarmeitbl/*.go
	( GOOS=linux GOARCH=amd64 go build -o $@ gocomandar.go )

builds/gocomandar.linux.arm64: gocomandar.go goalarmeitbl/*.go
	( GOOS=linux GOARCH=arm64 go build -o $@ gocomandar.go )

builds/gocomandar.darwin.amd64: gocomandar.go goalarmeitbl/*.go
	( GOOS=darwin GOARCH=amd64 go build -o $@ gocomandar.go )

builds/gocomandar.darwin.arm64: gocomandar.go goalarmeitbl/*.go
	( GOOS=darwin GOARCH=arm64 go build -o $@ gocomandar.go )
