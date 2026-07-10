# Seguridad

ZenithDTL modela un protocolo de liquidacion local donde operadores reciben
capacidad segun reputacion, stake y rendimiento reciente. La superficie de
revision se centra en accounting, rutas, ventanas de epoch, withdrawals y
controles de admision.

## Modelo

- Las obligaciones se abren contra una ruta registrada y un asset unico.
- La ruta debe conectar origen y destino, respetar limite de hops y no superar
  la politica de fees.
- Cada canal mantiene limite, uso, reservas e importe en vuelo.
- La reputacion del operador se expresa como score de 0 a 1000 y banda publica.
- Los withdrawals de boveda usan una cola con epoch de espera antes del pago.

## Invariantes Esperadas

- Ningun canal debe reportar deuda activa por encima de su limite.
- Las rutas deben usar canales activos y operadores habilitados.
- Las obligaciones cerradas deben liberar o consumir la reserva correspondiente.
- Los withdrawals solo deben pagarse cuando esten listos.
- El reporte publico debe conservar balances no negativos y una traza auditable.

## Validacion Automatizada

Los tests TypeScript cubren:

- contrato del CLI y reportes JSON;
- reputacion y asignacion de capacidad;
- consumo visible de limites;
- penalizaciones de rendimiento;
- validacion de rutas y withdrawals.

Ejecutar:

```bash
bash scripts/ci.sh
```

## Dependencias

El nucleo Go no usa dependencias externas. Node solo se usa para scripts y
tests de integracion. Dependabot cubre `gomod`, `npm` y GitHub Actions.

## Reportes Internos

Los hallazgos deben incluir escenario, comando reproducible, impacto economico,
componentes afectados y propuesta de mitigacion. No incluir datos privados ni
credenciales en issues, logs o fixtures.
