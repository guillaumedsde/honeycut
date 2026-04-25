FROM golang:1.26-alpine AS build

USER 1000

WORKDIR /app

COPY . .

ARG GOCACHE=/gocache

RUN --mount=type=cache,target=/gocache,uid=1000,gid=1000 \
    go build

FROM scratch

USER 1000

COPY --from=build --chmod=755 --chown=0:0 /app/honeycut /honeycut

EXPOSE 8080

ENTRYPOINT [ "/honeycut" ]
