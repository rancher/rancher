git fetch > /dev/null 2>&1 
mkdir -p .build/v3
cp ./scripts/generate_old.gofile ./scripts/generate_new.gofile ./scripts/compare.gofile .build/
mv .build/generate_old.gofile .build/generate_old.go
mv .build/generate_new.gofile .build/generate_new.go
mv .build/compare.gofile .build/compare.go
test -z $(go run .build/generate_new.go)
mv .build/images.json .build/new-images.json
latestTag=$(git describe --tags --abbrev=0 | cut -f1 -d "-")
git show $latestTag:./vendor/github.com/rancher/types/apis/management.cattle.io/v3 | awk '{if(NR>2)print}'  | grep -v "zz_" | grep ".go" | while read filename; do
	git show $latestTag:./vendor/github.com/rancher/types/apis/management.cattle.io/v3/$filename > .build/v3/$filename
done
test -z $(go run .build/generate_old.go)
mv .build/images.json .build/old-images.json
go run .build/compare.go
if [ $? -eq "1" ]; then
    exit 1
fi

