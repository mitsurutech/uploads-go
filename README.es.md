# Subida de archivos en Go: S3 para los bytes, Postgres para todo lo demás

[English](README.md) | [Français](README.fr.md) | [Español](README.es.md) | [日本語](README.ja.md) | [Deutsch](README.de.md)

Una aplicación de ejemplo para la [plataforma de aprendizaje MitsuruTech](https://mitsurutech.co.uk/es). Subes un
archivo; S3 lo guarda; Postgres recuerda adónde fue y cómo se llama. La tabla de la página
sale de Postgres, y cada descarga sale de S3.

Es pequeña a propósito, cuatro rutas y tres archivos, para que puedas leerla entera antes de
cambiar nada.

```
main.go    la web: cuatro rutas y nada más
store.go   los dos almacenes, y el orden en que se escribe en ellos
page.go    el HTML
```

## Desplegarla aquí

Abre **Deploy** en la plataforma, elige este repositorio y arranca una **base de datos** y
la **pila cloud** a su lado. Todo lo que la aplicación necesita ya está entonces en su
entorno; no hay nada que configurar.

| Variable | La define | Qué es |
| --- | --- | --- |
| `DATABASE_URL` | tu sesión | el servidor Postgres que se ejecuta como `db` |
| `AWS_ENDPOINT_URL` | tu sesión | la pila cloud que se ejecuta como `cloud` |
| `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` | tu sesión | las credenciales para acceder a ella |
| `PORT` | la plataforma | el puerto en el que escuchar |
| `BUCKET_NAME` | tú, si quieres | por defecto `uploads` |

La plataforma la compila con `go build -o app .` y la arranca con `./app`.

## Las tres cosas que merece la pena leer

**1. `o.UsePathStyle = true`, en `store.go`.** Es el ajuste que todos pasan por alto
primero. Por defecto, el SDK de AWS pone el bucket en el nombre de host, `uploads.cloud`, y
no existe ese nombre en la red de tu sesión, así que cada llamada falla con un error de DNS
que no dice nada de buckets. El estilo «path» pide en su lugar `http://cloud:4566/uploads/...`.

**2. La aplicación crea su propio bucket.** Tu pila cloud arranca completamente vacía,
igual que una cuenta de AWS de verdad recién abierta. `CreateBucket` es trabajo de tu
código, se ejecuta en cada arranque y acepta «ya eres el propietario de este», para que un
reinicio no sea un fallo.

**3. Escribe primero en S3 y después la fila. Borra primero la fila y después S3.**

- Sube los bytes y luego inserta la fila. Si la inserción falla, queda un objeto al que nadie
  apunta: invisible, inofensivo, fácil de barrer.
- Si escribes primero la fila y la subida falla, queda en la página un archivo que no se
  puede abrir, y no hay forma de distinguir esa fila de una buena.
- El borrado va en sentido contrario por la misma razón.

Uno de esos fallos son unos pocos bytes perdidos. El otro es una página rota. Deja siempre
pasar primero lo que puede quedar huérfano sin daño.

## Dos hábitos de seguridad, tomados en serio en un ejemplo a propósito

**La clave de S3 es un uuid, nunca el nombre del archivo.** Dos personas suben `notes.txt` y
una no debe sobrescribir a la otra; y un nombre que viene de un navegador no es algo con lo
que construir una ruta (`../../etc/passwd` es un nombre de archivo perfectamente válido para
un formulario). El nombre real es una columna, que es donde está a salvo.

**Las descargas son siempre `attachment`, siempre `application/octet-stream`, siempre
`nosniff`.** Cualquiera puede subir cualquier cosa. Un archivo HTML servido en línea se
ejecutaría como una página en la propia dirección de tu aplicación, con todo lo que esa
página pueda alcanzar; guardado en el disco, es inerte.

Y cada nombre de archivo se escapa al volver a la página: un archivo llamado
`<img src=x onerror=alert(1)>` es un nombre de archivo perfectamente bueno y no debe
convertirse en marcado.

## Ejecutarla en tu propio equipo

Necesitas Go, un Postgres y algo con forma de S3. Con Docker:

```bash
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=dev --name pg postgres:16
docker run -d -p 4566:4566 --name cloud localstack/localstack

export DATABASE_URL=postgres://postgres:dev@localhost:5432/postgres
export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1

go build -o app . && ./app
```

Después abre <http://localhost:8080>.

## Cosas que probar a continuación

- Enviar una subida grande directamente a S3 en lugar de tenerla en memoria (`main.go` dice
  dónde).
- Añadir un contador de descargas, en DynamoDB: tu pila cloud también lo emula.
- Poner los archivos detrás de un inicio de sesión, para que la tabla sea de cada persona.
- Mostrar una vista previa de imagen, usando una URL prefirmada en lugar de pasar los bytes
  por la aplicación.

## Más información

- [Desplegar una aplicación desde GitHub](https://mitsurutech.co.uk/es/help/deploy-app):
  cómo ejecuta la plataforma un repositorio como este.
- [La pila cloud](https://mitsurutech.co.uk/es/help/cloud-stack): el servicio con forma de
  S3 que tu sesión arranca para que lo use la aplicación.
- ¿Das clase? [El primer año es gratis](https://mitsurutech.co.uk/es/pricing) para hasta 50
  alumnos, y [registrarse](https://mitsurutech.co.uk/es/signup) solo pide una dirección de
  correo electrónico.

## Acerca de

Hecho por [Mitsuru Technologies Ltd](https://mitsurutech.co.uk/es/learning) (registrada en Inglaterra y Gales, número de empresa 16755264), que crea la
[plataforma de aprendizaje MitsuruTech](https://mitsurutech.co.uk/es/learning) para escuelas y centros.

El mismo ejemplo en otros lenguajes: [Node](https://github.com/mitsurutech/uploads-node),
[Python](https://github.com/mitsurutech/uploads-python) y
[Java](https://github.com/mitsurutech/uploads-java).

## Licencia

Esta obra está bajo licencia [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/). Puedes compartirla y adaptarla para aprender y enseñar, si citas a Mitsuru Technologies Ltd, no la usas comercialmente y compartes lo que hagas con ella bajo la misma licencia. El texto completo está en el archivo `LICENSE`.
