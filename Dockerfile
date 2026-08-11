FROM golang AS build

COPY . /code
WORKDIR /code
RUN go build -o mirror_filter ./cmd/mirror_filter

FROM ubuntu
RUN apt-get update && apt-get upgrade -y && apt-get install -y ca-certificates
COPY --from=build /code/mirror_filter /usr/local/bin/mirror_filter
RUN useradd --system --home /home/mirror_filter --user-group mirror_filter
USER mirror_filter
WORKDIR /home/mirror_filter

RUN mkdir -p /home/mirror_filter/filter
COPY filter.lua /home/mirror_filter/filter/filter.lua
COPY config.template.yml /home/mirror_filter/config.yml

ENTRYPOINT ["/usr/local/bin/mirror_filter"]

