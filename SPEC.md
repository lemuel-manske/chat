# Especficicação

## Objetivo:

Evoluir a aplicação de chat vista em aula, que conecta apenas dois pares, para um sistema de conversa entre N participantes sem servidor central.

Cada instância da aplicação é, ao mesmo tempo, servidor e cliente: escuta conexões de entrada e estabelece conexões de saída. Não existe nó privilegiado, nó coordenador nem retransmissor central.

## Ponto de partida:

Ver [especificação](./SPEC.md).

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
