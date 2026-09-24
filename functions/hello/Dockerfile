FROM golang:1.22 AS build
WORKDIR /src
COPY main.go go.mod ./
RUN CGO_ENABLED=0 go build -o /out/hello-function .

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/hello-function /hello-function
EXPOSE 8080
ENTRYPOINT ["/hello-function"]
