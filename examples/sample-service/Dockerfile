# Hand-written Dockerfile for the M1 deterministic pipeline target.
# Multi-stage: build a static binary, ship it on distroless.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/sample-service .

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/sample-service /sample-service
EXPOSE 8080
ENTRYPOINT ["/sample-service"]
