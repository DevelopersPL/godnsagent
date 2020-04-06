FROM golang AS builder
COPY . src/godnsagent/
WORKDIR src/godnsagent/
RUN go mod download
RUN CGO_ENABLED=0 go build -ldflags "-X 'main.date=$(date)' -X 'main.version=$(git log --pretty=format:'%h' -n 1)'"

FROM alpine
COPY --from=0 /go/src/godnsagent/godnsagent /usr/bin/godnsagent
RUN chmod +x /usr/bin/godnsagent
CMD /usr/bin/godnsagent
