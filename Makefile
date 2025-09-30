.PHONY: generate_cert
generate_cert:
	cd cert && \
    openssl genrsa -out server.key 2048 && \
    openssl req -x509 -new -nodes -key server.key \
      -sha256 -days 3650 \
      -out server.crt -config cert.conf

.PHONY: clean_cert
clean_cert:
	rm -f cert/server.key cert/server.crt