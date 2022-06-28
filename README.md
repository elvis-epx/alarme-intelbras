# Receptor IP Intelbras

O objetivo principal deste projeto é implementar um "Receptor IP" alternativo
para centrais de alarme Intelbras.

Um objetivo secundário do projeto é fornecer uma documentação alternativa
para os protocolos dessas centrais de alarme, no espírito "code-as-documentation".

## Estado da implementação

No momento, o receptor implementa o protocolo da forma mais correta
possível (até onde vai nosso entendimento) e interpreta todos os eventos
listados na documentação (arme, desarme, disparo de zona, fechamento de
zona, etc.), reportando mensagens legíveis.

O programa também é capaz de fazer o download de fotos de disparos do sensor
IVP-8000.

## Plataforma e hardware de alarme

O programa é desenvolvido e testado nos sistemas operacionais Linux
e macOS, com ênfase maior no Linux, pois nosso caso de uso é um
"Receptor IP" rodando na nuvem.

Em nosso projeto, fazemos testes apenas com a central AMT-8000, que
é a que possuímos, embora o
protocolo pareça ser o mesmo para todas as centrais monitoradas via Internet.

Uma vez que o protocolo não é criptografado, utilizamos VPN entre alarme e
Receptor, e recomendamos que você faça o mesmo!

## Limitações atuais

No momento o receptor é apenas um programa de linha de comando, que reporta
os eventos na saída de terminal. No futuro próximo, disponibilizaremos um
gancho de integração.

O programa não tem pacote de instalação nem imagem Docker. No momento, é
necessário que o usuário possua conhecimento suficiente de informática.

O receptor não verifica a versão de Python instalada em seu sistema, mas
deve ser razoavelmente atual (mínimo absoluto 3.5, recomendado 3.8 ou acima).

Ao fazer a conexão de callback para download de fotos, o Receptor presume que
o IP da central é o mesmo da conexão principal. Isto pode não funcionar se
houver um roteador NAT no caminho (este é outro motivo pelo qual usamos VPN).

## Como rodar

Invoque o programa via linha de comando

```
./receptorip <porta> <porta central> <senha> <tamanho senha>
```

"Porta" é a porta TCP/IP em que o Receptor IP aceita conexão.
Se passado o valor 0, ouve na porta default 9009. (Pode-se usar
qualquer número de porta, desde que a respectiva configuração seja atualizada
na central.)

"Porta central" é a porta TCP/IP em que a central aceita conexão.
Se passado o valor 0, usa a porta default 9009. Esta porta é utilizada
para conexões de callback, utilizadas apenas obter fotos de um disparo de zona
do sensor IVP-8000.

"Senha" é a senha de acesso remoto (usuário 98), a mesma utilizada
para acesso à central via app AMT Mobile. Utilizada apenas para obter
fotos do sensor IVP-8000. Informe um número qualquer se não deseja fazer
download de fotos.

"Tamanho senha" é o tamanho da senha acima. Informe 4 ou 6, ou zero se
não deseja que seja feito download de fotos.

Exemplo:

```
./receptorip 9010 0 123456 6
```

## Roadmap

- Gancho de integração de eventos com programas externos (para envio de SMS, e-mail, etc.).
- Exemplos de integrações (e-mail e PushOver pelo menos).
- Integração de eventos com foto.
- Utilitário para obtenção manual de fotos de eventos
- Hierarquização de eventos - todas as mensagens nível log.info são importantes mas algumas podem ser enviadas em bloco, e não precisam ir imediatamente.
- Rodar como serviço (daemon) em background.
- Testes unitários e de robustez.
- Script de restart em caso de quebra
- Monitor externo de funcionamento
- Configuração em arquivo em vez de parâmetros CLI


## Motivação

Numa configuração "baunilha", o Receptor IP é uma empresa de segurança
e monitoramento, que provavelmente está rodando o software homônimo da
Intelbras.

Porém, existem casos em que pode ser útil haver um "Receptor IP"
alternativo, por exemplo

a) numa área onde nenhuma empresa de segurança possa atender, porém
os eventos ainda poderiam ser reportados para uma rede de vizinhos;

b) seja desejável armazenar e/ou tratar os eventos de alarme
de forma sistemática e automatizada, em particular quando o celular do usuário
está fora de área. Envio de e-mail, SMS (não mais suportado no firmware
da central) e integração WhatsApp podem ser feitos via nuvem.

c) um caso particular do ponto (b) é o report de sensores de movimento
capazes de tirar fotos. Um invasor diligente procurará destruir
a central para eliminar essas fotos. Um Receptor IP rodando na nuvem 
coloca os eventos e as fotos a salvo, e fora de alcance.

d) uma central de alarme poderia ser usada em projetos IoT não 
necessariamente relacionados com segurança patrimonial.

e) o Receptor IP da Intelbras é um software Windows, o que nem sempre
é conveniente. A nossa alternativa rodaria facilmente num Raspberry Pi Zero.

Outra motivação para este projeto, mais pessoal, é conhecer mais de perto
esse ecossistema das centrais de alarme. Os protocolos são verdadeiras 
cápsulas do tempo; suas implementações possuem cacoetes dos tempos em que
eventos de alarme eram reportados por DTMF, portas seriais e modem discado.

## Onde está a documentação dos protocolos?

Os documentos do protocolo Intelbras não são públicos, por isso não
há cópias deles aqui neste repositório. Mas eles mas podem ser obtidos
facilmente acionando o suporte via WhatsApp. Instruções em
https://forum.intelbras.com.br/viewtopic.php?f=2257&t=73067

Não é preciso assinar nenhum acordo de confidencialidade
para acessar ou ler esses documentos. Portanto, fica implicitamente
permitido que este projeto (e outros) possa ser de código-fonte aberto.
(O que na prática serve como documentação alternativa do protocolo.)

Aproveitamos para agradecer ao suporte Intelbras, em particular 
Robson dos Santos, pela pronta resposta e fornecimento de informações, nas
diversas ocasiões em que precisamos de direções e esclarecimentos.
