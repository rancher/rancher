# setup and teardown for tests
setup_environment() {
    rm -r .home/
    mkdir .home/

    # clear journal entries
    curl -X "DELETE" http://localhost:8888/api/v2/journal
}
