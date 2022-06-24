O objetivo deste projeto é implementar um "Receptor IP" alternativo
para centrais de alarme Intelbras.

O report de eventos via IP é a tendência geral do mercado de alarmes,
pois o telefone fixo está a caminho da extinção. Até mesmo o report
via SMS foi removido em firmwares mais recentes da central AMT-8000.
A central tem bateria interna e pode usar GPRS/3G além de Ethernet, o
que supre os casos de queda de energia e interrupção da conexão à Internet.

Numa configuração "baunilha", o Receptor IP é uma empresa de segurança
e monitoramento, que provavelmente está rodando o software homônimo da
Intelbras.

Em paralelo, o usuário pode interagir com a central através
do aplicativo AMT Mobile v3. Uma vez que a conexão acontece através
da Intelbras Cloud, não é necessário preocupar-se com NAT, IP dinâmico, etc.

Porém, existem casos em que pode ser útil haver um "Receptor IP"
alternativo, por exemplo

a) numa área onde nenhuma empresa de segurança possa atender, porém
os eventos ainda poderiam ser reportados para uma rede de vizinhos;

b) seja desejável armazenar e/ou tratar os eventos de alarme
de forma sistemática e automatizada, em particular quando o celular do usuário
está fora de área. Envio de e-mail ou até SMS (não mais suportado no firmware
da central) pode ser executado via nuvem.

c) um caso particular do ponto (b) é o report de sensores de movimento
capazes de tirar fotos. Um invasor diligente procurará destruir
a central para eliminar essas fotos. Então é importante que haja um Receptor IP
configurado para que as fotos sejam salvas a tempo.

d) uso da central de alarme em usos atípicos, não relacionados a segurança
patrimonial.

Em nosso projeto, testamos com a
central AMT-8000, embora o protocolo pareça ser o mesmo para todas
as centrais monitoradas via Internet.

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
