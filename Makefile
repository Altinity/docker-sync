all:
	mkdir dist 2>/dev/null || true
	cd cmd/docker-sync && go build -o ../../dist/docker-sync
	echo "Binary is in dist/docker-sync"
