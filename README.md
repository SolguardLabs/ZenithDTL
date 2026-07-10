# ZenithDTL

![banner](./assets/banner.png)

ZenithDTL es un simulador Go de liquidacion DTL basado en reputacion de
operadores. El protocolo asigna capacidad de settlement segun score historico,
stake operativo, estado del operador y canales activos entre dominios.

El repositorio modela un entorno local de auditoria con CLI, reportes JSON,
tests TypeScript y escenarios reproducibles.

## Componentes

- `src/ledger.go`: cuentas, bovedas, reservas, creditos y withdrawals.
- `src/reputation.go`: scorecards, ventanas de rendimiento y ajustes diferidos.
- `src/operators.go`: operadores, canales y distribucion de capacidad.
- `src/routes.go`: rutas multi-hop, fees y validacion de conectividad.
- `src/settlement.go`: apertura, liquidacion, expiracion y cancelacion de obligaciones.
- `src/epoch.go`: cierre de epoch, planificacion y refresco de limites.
- `cmd/zenithdtl`: CLI para escenarios y validacion.

## Requisitos

- Go 1.22 o superior.
- Node.js 22 o superior para los tests TypeScript.
- Bash para ejecutar los scripts de CI local.

## Uso

```bash
node scripts/build.mjs
./out/zenithdtl list
./out/zenithdtl run balanced-reputation --json
./out/zenithdtl validate route-withdrawal
```

En Windows el binario se genera como `out/zenithdtl.exe`.

## Tests

```bash
bash scripts/tests.sh
```

El comando compila el binario Go, ejecuta `go test ./...` y lanza los tests
TypeScript con `node --experimental-strip-types --test`.

## Escenarios

- `balanced-reputation`: liquidacion con operadores de bandas distintas.
- `capacity-limits`: admision de obligaciones contra canales hot y warm.
- `penalty-cycle`: ventanas de settlement expiradas y score operativo ajustado.
- `route-withdrawal`: rutas multi-hop con ciclo completo de withdrawals.

## Estructura

```text
.
├── cmd/zenithdtl
├── src/
├── tests/
├── scripts/
├── .github/
├── .vscode/
├── go.mod
├── package.json
├── README.md
└── SECURITY.md
```

## Estado

Proyecto de laboratorio local. No requiere servicios externos ni credenciales.
