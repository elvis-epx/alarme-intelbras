PROGRAMAS = goreceptor gocomandar
PLATAFORMAS = linux.amd64 linux.arm64 darwin.amd64 darwin.arm64

BINARIOS = $(foreach p,$(PROGRAMAS),$(foreach s,$(PLATAFORMAS),builds/$(p).$(s)))
FONTES = $(wildcard goalarmeitbl/*.go) go.mod go.sum

all: $(BINARIOS)

clean:
	rm -f builds/*

test:
	go vet ./...
	go test -race ./...

# builds/<programa>.<os>.<arch>
builds/%: $(FONTES) $(wildcard cmd/*/*.go)
	GOOS=$(word 2,$(subst ., ,$*)) GOARCH=$(word 3,$(subst ., ,$*)) \
		go build -trimpath -o $@ ./cmd/$(word 1,$(subst ., ,$*))

.PHONY: all clean test
