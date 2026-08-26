# Chat

CLI de chat peer-to-peer desenvolvida em Go, com foco em simplicidade, portabilidade e configuração objetiva.

## Sobre

O projeto permite executar diferentes instâncias de chat a partir de arquivos de configuração em YAML.

A aplicação foi criada para explorar comunicação entre peers utilizando uma base simples, fácil de executar e modificar.

## Requisitos

Para executar a partir do código-fonte:

- Go 1.25+
- Git

## Instalação

Clone o repositório:

```bash
git clone https://github.com/lemuel-manske/chat.git
cd chat
```

Instale as dependências e compile:

```bash
go mod download
go build -o chat .
```

## Uso

Execute a aplicação informando um arquivo de configuração:

```bash
./chat configs/config_kaue.yaml
```

No Windows:

```powershell
.\chat.exe configs\config_kaue.yaml
```

Os arquivos de configuração ficam no diretório `configs/`.

## Desenvolvimento

Executar sem gerar um binário:

```bash
go run . configs/config_kaue.yaml
```

Executar verificações do projeto:

```bash
go vet ./...
go test ./...
```

## Releases

Versões compiladas para diferentes sistemas operacionais podem ser disponibilizadas pela página de [Releases](https://github.com/lemuel-manske/chat/releases).

## Política para participantes lentos

Cada conexão possui uma fila de saída limitada e uma `goroutine` dedicada à escrita no socket.

O envio para os participantes é não bloqueante; Se a fila de saída de um participante atingir 
o limite configurado, esse participante é considerado lento e sua conexão é encerrada.

Essa política foi escolhida para impedir que um participante que pare de consumir mensagens 
bloqueie ou aumente a latência da conversa dos demais participantes.
