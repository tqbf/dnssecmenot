APP = dnssecmenot
SRC = $(shell find . -name '*.go')
CSS_SRC = tailwind.config.js $(shell find assets -name '*.css')
NODE_MODULES = node_modules
CSS_OUT = static/styles.css

dnssecmenot: $(SRC)
	go build -o $(APP) .

$(NODE_MODULES): package.json package-lock.json
	npm install

$(CSS_OUT): $(CSS_SRC) $(NODE_MODULES)
	npm run build:css

.PHONY: all clean
all: $(APP) $(CSS_OUT)

css: $(CSS_OUT)

app: $(APP)


clean:
	rm -f $(APP) $(CSS_OUT)
