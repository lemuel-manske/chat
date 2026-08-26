# Especficicação

## Objetivo:

Evoluir a aplicação de chat vista em aula, que conecta apenas dois pares, para um sistema de conversa entre N participantes sem servidor central.

Cada instância da aplicação é, ao mesmo tempo, servidor e cliente: escuta conexões de entrada e estabelece conexões de saída. Não existe nó privilegiado, nó coordenador nem retransmissor central.

## Ponto de partida:

A aplicação `SocketChat`, disponibilizada em aula, estabelece uma conexão TCP direta entre exatamente dois pares, com delimitação de mensagens por prefixo de tamanho.
Vocês podem reaproveitar integralmente o código de framing e de estabelecimento de conexão. O que muda é tudo o que decorre de existirem muitos pares e de nenhum deles ter uma visão
privilegiada do sistema.

### Arquitetura exigida:
- Toda instância abre uma porta de escuta e aceita conexões de entrada.
- Toda instância pode iniciar conexões de saída para outros pares.
- A comunicação entre dois pares acontece diretamente, sem intermediário.
- Não é permitido eleger um nó como servidor, coordenador ou repositório da lista de participantes.

Restrição de tecnologia: apenas a classe Socket sobre TCP. Não é permitido usar SignalR, gRPC, WebSocket, MQTT, bibliotecas de P2P prontas, filas de mensagens ou banco de dados compartilhado

## Requisitos:

1. A aplicação recebe, por argumento ou arquivo de configuração, sua porta de escuta, seu apelido e a lista de pares conhecidos.
2. Ao iniciar, o par conecta-se aos pares conhecidos e passa a aceitar conexões dos demais, formando uma malha completa.
3. Uma mensagem digitada por qualquer participante é entregue a todos os demais.
4. Cada mensagem exibida identifica seu autor.
5. As mensagens são delimitadas corretamente. Uma mensagem longa ou uma rajada de mensagens curtas não pode produzir texto truncado nem mensagens grudadas.
6. Toda operação de rede possui prazo definido.
7. A queda de um par não derruba nem trava os demais.
8. Quando um par encerra, seja de forma limpa ou abrupta, os demais o removem da lista de participantes e anunciam a saída.
9. Um participante que pare de consumir mensagens não pode travar a conversa dos outros. A política adotada (descartar, enfileirar com limite, ou desconectar) deve estar documentada e justificada.
10. Comando /list exibindo os participantes atualmente conhecidos, e /quit para sair anunciando a saída.
11. Mensagem privada com /msg apelido texto, entregue diretamente ao destinatário
