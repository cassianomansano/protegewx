# Aviso legal e de marcas

## Projeto independente

O ProtegeWX é um projeto **independente e não oficial**. Não possui vínculo,
patrocínio, aprovação nem qualquer relação com a PC SOFT, com a Thales, com a
SafeNet ou com seus sucessores e afiliadas.

## Marcas de terceiros

WINDEV, WEBDEV, WINDEV Mobile e PC SOFT são marcas de seus respectivos titulares.
Sentinel, Sentinel HL, SafeNet e HASP são marcas da Thales / SafeNet.

Esses nomes aparecem neste projeto **exclusivamente de forma descritiva**, para
identificar com quais produtos a ferramenta interopera. Não há uso de logotipos,
identidade visual, nem qualquer elemento que sugira origem, endosso ou afiliação.

## O que este software faz e não faz

Este é o ponto central, e é verificável lendo o código-fonte, que é aberto.

**O ProtegeWX se limita a:**

- alterar configuração de rede do próprio sistema operacional (regras do Firewall
  do Windows, arquivo `hosts`);
- alterar o arquivo de configuração `hasplm.ini`, do Sentinel License Manager,
  **usando opções documentadas e oferecidas pelo próprio produto** em seu painel
  de administração;
- ler informação de estado que o Sentinel License Manager já expõe publicamente
  em `http://127.0.0.1:1947`;
- aplicar permissões de arquivo (ACL) do próprio Windows;
- registrar em disco o estado das licenças, para efeito de comparação.

**O ProtegeWX NÃO:**

- quebra, contorna, remove ou enfraquece qualquer mecanismo de proteção de cópia;
- emula, clona, falsifica ou simula dongle, chave, licença ou resposta de licença;
- realiza engenharia reversa de código protegido;
- modifica, corrige (*patch*) ou redistribui qualquer binário de terceiros;
- contém, embute ou redistribui código, biblioteca, driver ou arquivo da PC SOFT,
  da Thales ou de qualquer terceiro;
- concede acesso a recurso, módulo ou funcionalidade que o usuário não tenha
  licenciado;
- estende, renova ou altera o conteúdo, o prazo ou o alcance de qualquer licença.

Nenhum dado é escrito dentro do dongle. Todas as alterações são no sistema
operacional do próprio usuário, e todas são reversíveis pelo próprio programa.

## Uso pretendido

Destina-se ao **titular do equipamento e da licença**, para controlar o tráfego de
rede que sai da sua própria máquina e para registrar o estado das licenças que
adquiriu.

## Sua responsabilidade

O contrato de licença que você assinou com o fornecedor **continua valendo**. Este
software não altera, não interpreta e não substitui esse contrato.

Antes de usar, considere que:

- pode haver cláusula contratual sobre atualizações, telemetria ou manutenção;
- bloquear atualizações significa **deixar de receber correções e service packs**;
- em ambiente corporativo, políticas internas de TI podem ser aplicáveis.

Se essas questões forem relevantes no seu caso, consulte o seu contrato e, se
necessário, assessoria jurídica. **A decisão e a responsabilidade pelo uso são de
quem usa.**

## Sem garantia

Fornecido "como está", sem garantia de qualquer natureza, expressa ou implícita,
conforme os termos da licença MIT que acompanha este projeto. Os autores e
contribuidores não respondem por qualquer dano decorrente do uso.

**Leia o que cada ação faz antes de aplicar.** O programa mostra o comando exato
antes de executar, e o comando `--status` apenas lê, sem alterar nada.
