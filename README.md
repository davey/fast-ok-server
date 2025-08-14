# Intro

This is what came out when I asked ChatGTP the following:

> "Please write a minimal high-performace webserver, that answers all requests with status 200 and body "OK". 
> This webserver needs to be able to handle many requests (mainly GET, POST, about 4000-5000 per second) and run on linux.
> Please chose a fast and reliable programming language that is well made for low ressource consumption and high performance.
> Ideally the webserver regularly outputs some statistics about how many requests/s und the average request size (in bytes) were handled.
> This webserver is meant to be a destination for debugging request mirroring of haproxy with spoa-mirror."

# Compile & Use

```
## Install Go (1.20+ recommended)
sudo apt-get update && sudo apt-get install -y golang

## Get deps
go env -w GOPATH="$HOME/go"
go install github.com/valyala/fasthttp

## Save the code from the canvas as fast_ok_server.go, then:
go mod init fast-ok-server
go mod tidy
go build -o fast-ok-server fast_ok_server.go

## Run (listens on :8080 by default)
./fast-ok-server
```
