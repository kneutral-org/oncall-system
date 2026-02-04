.PHONY: proto proto-lint proto-breaking proto-format

proto:
	buf generate proto

proto-lint:
	buf lint proto

proto-breaking:
	buf breaking proto --against '.git#branch=main'

proto-format:
	buf format -w proto
