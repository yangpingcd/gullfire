# Pull base image.
FROM golang


# Define working directory.
WORKDIR /go/bin

# fetch
COPY . /go/src/github.com/yangpingcd/gullfire

# fetch the dependencies
#RUN go get github.com/vharitonsky/iniflags

# build the gullfire project
#RUN go build gullfire

RUN go get github.com/yangpingcd/gullfire


# Expose ports.
EXPOSE 80
