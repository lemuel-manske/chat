# chat

Esse repositório implementa um sistema de chat em tempo real utilizando tecnologias modernas.
O objetivo é fornecer uma plataforma onde os usuários possam se comunicar instantaneamente,
seja em grupos ou em conversas privadas.

## Como usar?

```bash
# Clone o repositório
git clone https://github.com/lemuel-manske/chat.git

# Acesse o diretório do projeto
cd chat

# Use o executável diretamente
./chat <config_file> # para linux
./chat-windows <config_file> # para windows

# Exemplo
./chat configs/config_kaue.yaml # inicia o chat com as configurações do arquivo config_kaue.yaml
```

## Gerando o executável

- certifique-se de ter o Go instalado

```bash
make build
```
