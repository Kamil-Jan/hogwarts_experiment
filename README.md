# Якубов Камиль Б05-223. Задача #4 / Системное программирование

## Компиляция

Добавить go в `PATH`
```
export GOPATH=$HOME/go
export PATH=$PATH:$GOPATH/bin
```

Склонировать репозиторий
```
git clone https://github.com/Kamil-Jan/hogwarts_experiment.git
cd hogwarts_experiment
```

Сгенерировать прото файлы
```
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
protoc --go_out=proto --go-grpc_out=proto proto/*.proto
```

# Запуск сервера

Для запуска сервера выполните `go run server/main.go`. Он запустится на порте `50051`. В логи будут писаться юзернеймы подключенных клиентов, полученные от них ответы, загаданное число.

Чтобы начать эксперимент выполните:
```
grpcurl -plaintext -d '{}' localhost:50051 experiment.ExperimentService.StartExperiment
```
Всем подключенным клиентам придет сообщение о начале эксперимента

Чтобы завершить эксперимент выполните:
```
grpcurl -plaintext -d '{}' localhost:50051 experiment.ExperimentService.EndExperiment
```
Сеанс подключенных клиентов завершится

Чтобы получить список юзеров, ожидающих ответ, выполните:
```
grpcurl -plaintext -d '{}' localhost:50051 experiment.ExperimentService.WaitingList
```

Чтобы получить таблицу лидеров, выполните:
```
grpcurl -plaintext -d '{}' localhost:50051 experiment.ExperimentService.Leaderboard
```
Она выдаст список участников с кол-вом экспериментов, где они угадали число

Чтобы отправить юзеру сообщение о том, угадал он число, или присланное им число больше / меньше загаданного, выполните
```
grpcurl -plaintext -d '{"username": "[name]"}' localhost:50051 experiment.ExperimentService.SendResponse
```

# Запуск клиента

Для запуска клиента выполните `go run client/main.go`. Нужно будет ввести свой юзернейм, после чего программа подключится к серверу и будет дожидаться начала эксперимента. После начала эксперимента можно будет начать угадывать число. Если число угадано или эксперимент закончился, программа завершает свою работу. Для повторного участия необходимо будет снова запустить `go run client/main.go`

# Дальнейшие улучшения

1. Добавить фронтенд
2. Хранить список лидеров в БД (например, в PostgreSQL)
3. Добавить возможность проводить несколько экспериментов одновремененно
4. Поднимать сервер на отдельном хосте или в докер контейнере
