FROM golang:1.11

COPY build/agogos-reverse-proxy /bin/agogos-reverse-proxy

CMD ["/bin/agogos-reverse-proxy"]