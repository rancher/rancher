TARGETS := $(shell ls scripts)

.dapper:
	@if [ `uname -s` = "Darwin" && `uname -m` = "arm64" ]; then \
		echo "Dapper download is not supported on ARM Macs, you need to build it and add it as .dapper in this directory";\
		exit 0;\
	else \
		echo Downloading dapper;\
		curl -sL https://releases.rancher.com/dapper/latest/dapper-`uname -s`-`uname -m` > .dapper.tmp;\
		chmod +x .dapper.tmp;\
		./.dapper.tmp -v;\
		mv .dapper.tmp .dapper;\
	fi

$(TARGETS): .dapper
	@if [ "$@" = "post-release-checks" ] || [ "$@" = "list-gomod-updates" ] || [ "$@" = "check-chart-kdm-source-values" ]; then \
		./.dapper -q --no-out $@;\
	else\
		./.dapper $@;\
	fi

.DEFAULT_GOAL := ci

.PHONY: $(TARGETS)
