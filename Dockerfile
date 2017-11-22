FROM golang
RUN go get -u github.com/golang/dep/cmd/dep
RUN go env
ADD . /go/src/github.com/bketelsen/gogrep
WORKDIR /go/src/github.com/bketelsen/gogrep
RUN make clean
RUN make
CMD /go/src/github.com/bketelsen/gogrep/bin/gogrep 
