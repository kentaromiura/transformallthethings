darwin: transform.go
	go build transform.go port_darwin.go

freebsd:
	make transform

linux: transform.go
	go build transform.go port_linux.go

netbsd: transform.go
	go build transform.go port_netbsd.go

openbsd: transform.go
	go build transform.go port_openbsd.go

transform: transform.go
	go build transform.go port_freebsd.go

example:
	umount mnt 2> /dev/null | true
	mkdir -p mnt
	./transform test mnt 2>/dev/null &
	sleep 1
	cd test; npm i 2> /dev/null
	test/node_modules/.bin/esbuild --bundle mnt/src/index.js > example.js
	umount mnt
	node example
	rm example.js
