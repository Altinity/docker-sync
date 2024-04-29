all:
	mkdir dist 2>/dev/null || true
	cd cmd/dockersync && go build -o ../../dist/dockersync
	echo "Binary is in dist/dockersync"