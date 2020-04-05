FROM golang AS builder
COPY . src/godnsagent/
WORKDIR src/godnsagent/
RUN go get
RUN CGO_ENABLED=0 go build -ldflags "-X 'main.BuildTime=$(date)' -X 'main.BuildVersion=$(git log --pretty=format:'%h' -n 1)'"

FROM alpine
COPY --from=0 /go/src/godnsagent/godnsagent .
RUN chmod +x godnsagent
CMD /godnsagent
