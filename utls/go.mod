module github.com/refraction-networking/utls

go 1.25.5

retract (
	v1.4.1 // #218
	v1.4.0 // #218 panic on saveSessionTicket
)

require (
	github.com/andybalholm/brotli v1.0.6
	github.com/klauspost/compress v1.17.4
	golang.org/x/crypto v0.50.0
	golang.org/x/net v0.52.0
	golang.org/x/sys v0.46.0
)

require golang.org/x/text v0.36.0 // indirect
