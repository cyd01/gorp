FROM    golang AS build
COPY    . /src
ARG     GOPROXY=${GOPROXY:-https://proxy.golang.org}
ARG     GOSUMDB=${GOSUMDB:-sum.golang.org}
ARG     GOOS=${GOOS:-linux}
ARG     GOARCH=${GOARCH:-386}
ARG     CGO_ENABLED=${CGO_ENABLED:-0}
RUN     cd /src && go mod tidy && go build -o /usr/local/bin/gorp ./cmd/gorp

FROM    alpine
COPY    --from=build /usr/local/bin/gorp /usr/local/bin/gorp
RUN     chmod +x /usr/local/bin/gorp
ENTRYPOINT ["/usr/local/bin/gorp"]