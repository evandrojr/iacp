#!/bin/bash

# release.sh - Script para gerar novas releases do iacp

if [ -z "$1" ]; then
    echo "Erro: Forneça a versão (ex: v1.0.1)"
    echo "Uso: ./release.sh v1.X.X"
    exit 1
fi

VERSION=$1

# 1. Rodar testes
echo "🚀 Rodando testes..."
go test ./...
if [ $? -ne 0 ]; then
    echo "❌ Testes falharam. Release abortada."
    exit 1
fi

# 2. Gerar Tag
echo "🏷️ Criando tag $VERSION..."
git tag -a "$VERSION" -m "Release $VERSION"
if [ $? -ne 0 ]; then
    echo "❌ Falha ao criar tag. Ela já existe?"
    exit 1
fi

# 3. Push para o GitHub
echo "📤 Enviando para o GitHub..."
git push origin main
git push origin "$VERSION"

echo "✅ Release $VERSION enviada com sucesso!"
echo "O GitHub Actions irá rodar o CI para validar o build."
echo "Nota: Se tiver o 'gh' CLI instalado, você pode rodar:"
echo "gh release create $VERSION --generate-notes"
