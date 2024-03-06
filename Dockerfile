FROM golang as build

COPY . /code
WORKDIR /code
RUN go build -o mirror_filter ./cmd/mirror_filter

FROM ubuntu
RUN apt-get update && apt-get upgrade -y && apt-get install -y ca-certificates
COPY --from=build /code/mirror_filter /usr/local/bin/mirror_filter
RUN adduser --system --home /home/mirror_filter --group --disabled-password mirror_filter
USER mirror_filter
WORKDIR /home/mirror_filter

ENTRYPOINT ["/usr/local/bin/mirror_filter"]

