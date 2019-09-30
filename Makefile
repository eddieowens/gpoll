.PHONY: mocks
mocks:
	mockery --output mocks --outpkg mocks --dir . --all --case snake