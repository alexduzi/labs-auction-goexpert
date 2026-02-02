# Sistema de Leilões - Fechamento Automático

Sistema de leilões em Go com funcionalidade de fechamento automático baseado em tempo configurável.

## 📋 Sobre o Projeto

Este projeto implementa um sistema de leilões com as seguintes funcionalidades:
- Criação de leilões
- Sistema de lances (bids) com processamento em batch
- **Fechamento automático de leilões** após tempo configurável
- API REST para gerenciamento
- Persistência em MongoDB

## 🎯 Funcionalidade Implementada: Fechamento Automático

O sistema agora conta com uma rotina automática que:
- Monitora leilões ativos continuamente
- Fecha automaticamente leilões cujo tempo expirou
- Utiliza goroutines para processamento assíncrono
- Garante thread-safety com mutex
- Tempo configurável via variável de ambiente

### Implementação Técnica

- **Arquivo principal**: `internal/infra/database/auction/create_auction.go`
- **Goroutine**: Executa verificação periódica (a cada metade do intervalo configurado)
- **Concorrência**: Uso de `sync.Mutex` para operações thread-safe
- **Batch update**: MongoDB `UpdateMany` para eficiência
- **Testes**: Cobertura completa com testcontainers

## 🚀 Tecnologias Utilizadas

- **Go 1.20+**
- **MongoDB** - Banco de dados
- **Gin** - Framework web
- **Docker & Docker Compose** - Containerização
- **Testcontainers** - Testes de integração

## 📦 Pré-requisitos

Antes de começar, certifique-se de ter instalado:

- [Docker](https://docs.docker.com/get-docker/) (versão 20.x ou superior)
- [Docker Compose](https://docs.docker.com/compose/install/) (versão 2.x ou superior)
- [Go](https://golang.org/dl/) 1.20+ (opcional, apenas para desenvolvimento local)

## 🔧 Configuração

### Variáveis de Ambiente

O projeto utiliza as seguintes variáveis de ambiente (configuradas em `cmd/auction/.env`):

```env
# Intervalo de processamento de lances em batch
BATCH_INSERT_INTERVAL=20s
MAX_BATCH_SIZE=4

# Tempo de duração dos leilões
AUCTION_INTERVAL=20s

# Configurações do MongoDB
MONGO_INITDB_ROOT_USERNAME=admin
MONGO_INITDB_ROOT_PASSWORD=admin
MONGODB_URL=mongodb://admin:admin@mongodb:27017/auctions?authSource=admin
MONGODB_DB=auctions
```

**Variável principal do desafio:**
- `AUCTION_INTERVAL`: Define quanto tempo um leilão permanece aberto (ex: `20s`, `5m`, `1h`)

## 🐳 Executando com Docker

### 1. Clone o repositório

```bash
git clone <url-do-repositorio>
cd fullcycle-auction_go
```

### 2. Inicie os containers

```bash
docker-compose up --build
```

A aplicação estará disponível em: `http://localhost:8080`

### 3. Verificar logs

```bash
# Logs da aplicação
docker-compose logs -f app

# Logs do MongoDB
docker-compose logs -f mongodb
```

### 4. Parar os containers

```bash
docker-compose down
```

### 5. Parar e remover volumes (limpar dados)

```bash
docker-compose down -v
```

## 💻 Executando Localmente (Desenvolvimento)

### 1. Instalar dependências

```bash
go mod download
```

### 2. Subir apenas o MongoDB

```bash
docker-compose up mongodb
```

### 3. Executar a aplicação

```bash
go run cmd/auction/main.go
```

## 🧪 Executando os Testes

### Testes Unitários e de Integração

```bash
# Todos os testes
go test ./... -v

# Testes específicos do fechamento automático
go test ./internal/infra/database/auction/... -v -count=1

# Com cobertura
go test ./... -cover

# Gerar relatório de cobertura
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Testes com Docker

```bash
# Executar testes dentro do container
docker-compose run --rm app go test ./... -v
```

## 📡 Endpoints da API

### Leilões (Auctions)

#### Criar Leilão
```bash
POST /auction
Content-Type: application/json

{
  "product_name": "iPhone 15 Pro",
  "category": "Eletrônicos",
  "description": "iPhone 15 Pro 256GB Azul",
  "condition": 1
}
```

**Condições disponíveis:**
- `1` - Novo
- `2` - Usado
- `3` - Recondicionado

#### Listar Leilões
```bash
GET /auction?status=0&category=Eletrônicos&productName=iPhone
```

#### Buscar Leilão por ID
```bash
GET /auction/:auctionId
```

#### Buscar Lance Vencedor
```bash
GET /auction/winner/:auctionId
```

### Lances (Bids)

#### Criar Lance
```bash
POST /bid
Content-Type: application/json

{
  "user_id": "550e8400-e29b-41d4-a716-446655440000",
  "auction_id": "660e8400-e29b-41d4-a716-446655440000",
  "amount": 1500.00
}
```

#### Listar Lances de um Leilão
```bash
GET /bid/:auctionId
```

### Usuários (Users)

#### Buscar Usuário por ID
```bash
GET /user/:userId
```

## 📖 Exemplos de Uso

### 1. Criar um leilão que expira em 30 segundos

```bash
# 1. Ajustar AUCTION_INTERVAL no .env
echo "AUCTION_INTERVAL=30s" >> cmd/auction/.env

# 2. Reiniciar aplicação
docker-compose restart app

# 3. Criar leilão via API
curl -X POST http://localhost:8080/auction \
  -H "Content-Type: application/json" \
  -d '{
    "product_name": "MacBook Pro",
    "category": "Eletrônicos",
    "description": "MacBook Pro M3 16GB 512GB",
    "condition": 1
  }'

# 4. Aguardar 30 segundos e verificar status
curl http://localhost:8080/auction/<auction_id>
```

### 2. Testar fechamento automático

```bash
# Criar leilão
AUCTION_ID=$(curl -X POST http://localhost:8080/auction \
  -H "Content-Type: application/json" \
  -d '{
    "product_name": "Teste",
    "category": "Teste",
    "description": "Leilão de teste para verificar fechamento",
    "condition": 1
  }' | jq -r '.id')

echo "Leilão criado: $AUCTION_ID"
echo "Aguardando fechamento automático..."

# Verificar status após AUCTION_INTERVAL
sleep 25
curl http://localhost:8080/auction/$AUCTION_ID | jq '.status'
# Deve retornar: 1 (Completed)
```

## 🏗️ Estrutura do Projeto

```
.
├── cmd/
│   └── auction/
│       ├── main.go              # Entry point da aplicação
│       └── .env                 # Variáveis de ambiente
├── configuration/
│   ├── database/
│   │   └── mongodb/             # Conexão com MongoDB
│   ├── logger/                  # Configuração de logs
│   └── rest_err/                # Tratamento de erros HTTP
├── internal/
│   ├── entity/                  # Entidades de domínio
│   │   ├── auction_entity/
│   │   ├── bid_entity/
│   │   └── user_entity/
│   ├── infra/
│   │   ├── api/web/            # Controllers e validações
│   │   └── database/           # Repositórios
│   │       ├── auction/        # ⭐ Fechamento automático implementado aqui
│   │       ├── bid/
│   │       └── user/
│   ├── internal_error/         # Erros internos
│   └── usecase/                # Casos de uso
├── docker-compose.yml
├── Dockerfile
└── README.md
```

## 🔍 Como Funciona o Fechamento Automático

### Fluxo de Execução

1. **Inicialização**: Ao criar o `AuctionRepository`, uma goroutine é iniciada automaticamente
2. **Monitoramento**: A goroutine verifica leilões expirados a cada `AUCTION_INTERVAL/2`
3. **Detecção**: Busca leilões com `status=Active` e `timestamp < (agora - AUCTION_INTERVAL)`
4. **Fechamento**: Executa `UpdateMany` para alterar status para `Completed`
5. **Logs**: Registra quantos leilões foram fechados

### Exemplo Visual

```
Tempo →

T=0s     | Leilão criado (Status: Active)
T=10s    | Verificação automática - Ainda ativo
T=20s    | ⏰ AUCTION_INTERVAL atingido
T=21s    | Verificação detecta expiração
         | ✅ Status alterado para Completed
```

## 🧪 Testes Implementados

### TestAutoCloseAuction
Verifica que:
- Leilões expirados são fechados automaticamente
- Leilões ativos permanecem abertos
- Fechamento ocorre no tempo correto

### TestAutoCloseAuctionAfterExpiration
Verifica que:
- Leilão criado começa como Active
- Após o intervalo configurado, status muda para Completed
- Transição de estado funciona corretamente

## 🐛 Troubleshooting

### Porta 8080 já em uso
```bash
# Identifique o processo
lsof -i :8080

# Mate o processo ou mude a porta no docker-compose.yml
```

### MongoDB não conecta
```bash
# Verificar logs
docker-compose logs mongodb

# Remover volumes e recriar
docker-compose down -v
docker-compose up
```

### Testes falhando
```bash
# Limpar cache de testes
go clean -testcache

# Executar com verbose
go test ./internal/infra/database/auction/... -v -count=1
```

### Leilões não fecham automaticamente
```bash
# Verificar logs da aplicação
docker-compose logs -f app

# Procurar por:
# - "Checking for expired auctions"
# - "Successfully closed expired auctions"
```

## 📊 Monitoramento

### Ver leilões ativos
```bash
docker exec -it mongodb mongosh -u admin -p admin

use auctions
db.auctions.find({status: 0}).pretty()
```

### Ver leilões fechados
```bash
db.auctions.find({status: 1}).pretty()
```

### Ver logs em tempo real
```bash
docker-compose logs -f app | grep "closed expired auctions"
```

## 📝 Notas Importantes

- O fechamento automático ocorre **assincronamente** via goroutine
- A verificação acontece a cada `AUCTION_INTERVAL/2` para maior precisão
- Usa `sync.Mutex` para garantir thread-safety
- Leilões expirados são fechados em **batch** para eficiência
- Suporta múltiplos leilões expirando simultaneamente

## 👥 Autores

Alex Duzi - [duzihd@gmail.com](mailto:duzihd@gmail.com)

---