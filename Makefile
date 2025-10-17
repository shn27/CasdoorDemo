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

.PHONY: replace_placeholders_into_same_file
replace_placeholders:
	@grep -v '^\s*#' .envrc | grep -v '^\s*$$' | while IFS='=' read -r key value; do \
		key=$$(echo "$$key" | sed 's/^[[:space:]]*//;s/[[:space:]]*$$//'); \
		value=$$(echo "$$value" | sed 's/^[[:space:]]*//;s/[[:space:]]*$$//'); \
		value=$$(echo "$$value" | sed 's/[",]//g'); \
		if [ -n "$$key" ] && [ -n "$$value" ]; then \
			sed -i "s|<<$$key>>|$$value|g" test.json; \
		fi; \
	done

.PHONY: replace_placeholders_into_new_file
replace_placeholders_into_new_file:
	cd Deploy && \
	chmod +x populate.sh && \
	./populate.sh




