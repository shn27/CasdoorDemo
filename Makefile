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

#.PHONY: replace_placeholders_into_new_file
#replace_placeholders_into_new_file:
#	@cp test.json test1.json 2>/dev/null || cp test.json test1.json; \
#	while IFS='=' read -r key raw_value; do \
#		# Skip empty lines or comments \
#		[ -z "$$key" ] && continue; \
#		echo "$$key" | grep -q '^[[:space:]]*#' && continue; \
#		# Trim spaces from key and value \
#		key=$$(echo "$$key" | sed 's/^[[:space:]]*//;s/[[:space:]]*$$//'); \
#		value=$$(echo "$$raw_value" | sed 's/^[[:space:]]*//;s/[[:space:]]*$$//' | sed 's/^[",]*//;s/[",]*$$//'); \
#		# Escape problematic chars for sed (/, \, &, |) \
#		escaped_value=$$(printf '%s' "$$value" | sed 's/[\\/&|]/\\&/g'); \
#		if [ -n "$$key" ] && [ -n "$$escaped_value" ]; then \
#			if grep -q "<<$$key>>" test1.json; then \
#				sed -i "s|<<$$key>>|$$escaped_value|g" test1.json; \
#				echo "✅ Replaced $$key"; \
#			else \
#				echo "⚠️  Placeholder <<$$key>> not found in test1.json"; \
#			fi; \
#		else \
#			echo "⚠️  Skipped invalid or empty entry: $$key=$$value"; \
#		fi; \
#	done < cmd/.env

.PHONY: replace_placeholders_into_new_file
replace_placeholders_into_new_file:
	@cp test.json test1.json 2>/dev/null || cp test.json test1.json; \
	# Create a temporary file for normalized key-value pairs \
	normalized_envrc=$$(mktemp); \
	\
	# Process .envrc to handle comma-separated key-value pairs and multi-line values \
	{ \
		tr -d '\n' < .envrc | tr ',' '\n'; \
	} | while IFS= read -r line; do \
		# Clean up the line and extract key-value pairs \
		cleaned_line=$$(echo "$$line" | sed 's/^[[:space:]]*//;s/[[:space:]]*$$//'); \
		if echo "$$cleaned_line" | grep -q '^[A-Z_][A-Z_0-9]*='; then \
			key=$$(echo "$$cleaned_line" | cut -d'=' -f1); \
			value=$$(echo "$$cleaned_line" | cut -d'=' -f2-); \
			# Remove surrounding quotes from value \
			value=$$(echo "$$value" | sed 's/^"//;s/"$$//;s/^'\''//;s/'\''$$//'); \
			# Remove any trailing commas that might remain \
			value=$$(echo "$$value" | sed 's/,[[:space:]]*$$//'); \
			echo "$$key=$$value"; \
		fi; \
	done > "$$normalized_envrc"; \
	\
	echo "Normalized environment variables:"; \
	cat "$$normalized_envrc"; \
	echo "------------------------"; \
	\
	# Now process the normalized file for replacement \
	while IFS='=' read -r key value; do \
		# Skip empty lines \
		[ -z "$$key" ] && continue; \
		\
		# Trim spaces from key and value \
		key=$$(echo "$$key" | sed 's/^[[:space:]]*//;s/[[:space:]]*$$//'); \
		value=$$(echo "$$value" | sed 's/^[[:space:]]*//;s/[[:space:]]*$$//'); \
		\
		# Skip if key or value is empty after trimming \
		[ -z "$$key" ] && continue; \
		[ -z "$$value" ] && continue; \
		\
		# Escape problematic chars for sed (especially important for multi-line certs) \
		escaped_value=$$(printf '%s' "$$value" | sed 's/[\/&]/\\&/g' | sed 's/|/\\|/g'); \
		\
		if [ -n "$$key" ] && [ -n "$$escaped_value" ]; then \
			if grep -q "<<$$key>>" test1.json; then \
				# Use perl for better multi-line handling instead of sed \
				perl -pi -e "s|<<$$key>>|$$escaped_value|g" test1.json; \
				echo "✅ Replaced <<$$key>>"; \
			else \
				echo "⚠️  Placeholder <<$$key>> not found in test1.json"; \
			fi; \
		fi; \
	done < "$$normalized_envrc"; \
	\
	rm -f "$$normalized_envrc"; \
	echo "✅ Replacement completed!"


