CC         = go
BUILD_PATH = ./bin
SRC        = ./main.go
TARGET     = snapchat-dl
BINS       = $(BUILD_PATH)/$(TARGET)

.PHONY: all clean build run

all: run

clean:
	rm -rf $(BUILD_PATH)

build: clean
	mkdir -p $(BUILD_PATH)
	$(CC) build -o $(BINS) $(SRC)

run:
	$(CC) run $(SRC)
