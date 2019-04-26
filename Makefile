
GO	=	/usr/bin/go
GO_FLAGS   = -v
GO_BUILD_FLAGS = $(GO_FLAGS)
GO_GET_FLAGS = $(GO_FLAGS) # add -u to update

SRCS = p-drop.go generate_keys.go network.go thumbnail.go
BINS = p-drop

all: deps bin

bin:
	$(GO) build $(GO_BUILD_FLAGS) $(SRCS)

# For release binary. Link statically, not position independent
static: deps
	$(GO) build -a -tags "static osusergo netgo" -ldflags '-s -extldflags "-fno-PIC -static"' $(SRCS)

dist: static
	strip $(BINS) # using -s at linking

deps:
	$(GO) get $(GO_GET_FLAGS) "github.com/julienschmidt/httprouter"
	$(GO) get $(GO_GET_FLAGS) "github.com/grandcat/zeroconf"
	$(GO) get $(GO_GET_FLAGS) "github.com/skip2/go-qrcode"
	$(GO) get $(GO_GET_FLAGS) "github.com/nfnt/resize"
