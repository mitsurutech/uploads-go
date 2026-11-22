# Datei-Uploads in Go: S3 für die Bytes, Postgres für alles andere

[English](README.md) | [Français](README.fr.md) | [Español](README.es.md) | [日本語](README.ja.md) | [Deutsch](README.de.md)

Eine Beispiel-App für die [MitsuruTech Learning Platform](https://mitsurutech.co.uk/de). Laden Sie eine Datei hoch; S3 bewahrt sie auf; Postgres merkt sich, wo sie liegt und wie sie heißt.
Die Tabelle auf der Seite kommt aus Postgres, und jeder Download kommt aus S3.

Sie ist absichtlich klein, vier Routen und drei Dateien, damit Sie das Ganze lesen können, bevor
Sie etwas daran ändern.

```
main.go    the web: four routes and nothing else
store.go   the two stores, and the order they are written in
page.go    the HTML
```

## Hier bereitstellen

Öffnen Sie auf der Plattform **Deploy**, wählen Sie dieses Repository und starten Sie daneben eine
**Datenbank** und den **Cloud-Stack**. Alles, was die App braucht, steht dann bereits in ihrer
Umgebung; es gibt nichts zu konfigurieren.

| Variable | Gesetzt von | Was sie ist |
| --- | --- | --- |
| `DATABASE_URL` | Ihrer Sitzung | der Postgres-Server, der als `db` läuft |
| `AWS_ENDPOINT_URL` | Ihrer Sitzung | der Cloud-Stack, der als `cloud` läuft |
| `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_REGION` | Ihrer Sitzung | die Zugangsdaten dafür |
| `PORT` | der Plattform | der Port, auf dem gelauscht wird |
| `BUCKET_NAME` | Ihnen, optional | Standard ist `uploads` |

Die Plattform baut sie mit `go build -o app .` und startet sie mit `./app`.

## Die drei Dinge, die sich zu lesen lohnen

**1. `o.UsePathStyle = true`, in `store.go`.** Das ist die Einstellung, die alle zuerst
übersehen. Standardmäßig setzt das AWS SDK den Bucket in den Hostnamen, `uploads.cloud`,
und diesen Namen gibt es im Netzwerk Ihrer Sitzung nicht. Jeder Aufruf scheitert also mit einem
DNS-Fehler, der nichts über Buckets sagt. Der Pfadstil fragt stattdessen nach
`http://cloud:4566/uploads/...`.

**2. Die App legt ihren Bucket selbst an.** Ihr Cloud-Stack startet völlig leer, genau wie ein
echtes AWS-Konto, das Sie gerade eröffnet haben. `CreateBucket` ist Aufgabe Ihres Codes, es läuft
bei jedem Start und akzeptiert „das gehört Ihnen schon“, damit ein Neustart kein Absturz ist.

**3. Zuerst S3 schreiben, dann die Zeile. Zuerst die Zeile löschen, dann S3.**

- Laden Sie die Bytes hoch, dann fügen Sie die Zeile ein. Scheitert das Einfügen, haben Sie ein
  Objekt, auf das niemand verweist: unsichtbar, harmlos, aufräumbar.
- Schreiben Sie zuerst die Zeile, hinterlässt ein gescheiterter Upload eine Datei auf der Seite,
  die sich nicht öffnen lässt, und keine Möglichkeit, diese Zeile von einer guten zu unterscheiden.
- Beim Löschen geht es aus demselben Grund andersherum.

Der eine Fehler kostet ein paar verschwendete Bytes. Der andere ist eine kaputte Seite. Lassen Sie
immer das zuerst gehen, was harmlos verwaisen kann.

## Zwei Sicherheitsgewohnheiten, in einem Beispiel absichtlich ernst genommen

**Der S3-Schlüssel ist eine UUID, nie der Dateiname.** Zwei Personen laden `notes.txt` hoch, und
die eine darf die andere nicht überschreiben; außerdem ist ein Name, der aus einem Browser kam,
nichts, woraus man einen Pfad baut (`../../etc/passwd` ist für ein Formular ein völlig gültiger
Dateiname). Der echte Name ist eine Spalte, und dort ist er sicher.

**Downloads sind immer `attachment`, immer `application/octet-stream`, immer `nosniff`.**
Jeder kann alles hochladen. Eine HTML-Datei, die inline ausgeliefert wird, liefe als Seite unter der
eigenen Adresse Ihrer App, mit allem, was diese Seite erreichen kann; auf der Festplatte gespeichert
ist sie harmlos.

Und jeder Dateiname wird auf dem Weg zurück auf die Seite maskiert: Eine Datei namens
`<img src=x onerror=alert(1)>` ist ein völlig korrekter Dateiname und darf nicht zu Markup werden.

## Auf dem eigenen Rechner ausführen

Sie brauchen Go, ein Postgres und etwas, das sich wie S3 verhält. Mit Docker:

```bash
docker run -d -p 5432:5432 -e POSTGRES_PASSWORD=dev --name pg postgres:16
docker run -d -p 4566:4566 --name cloud localstack/localstack

export DATABASE_URL=postgres://postgres:dev@localhost:5432/postgres
export AWS_ENDPOINT_URL=http://localhost:4566
export AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1

go build -o app . && ./app
```

Öffnen Sie dann <http://localhost:8080>.

## Was Sie als Nächstes ausprobieren können

- Einen großen Upload direkt nach S3 streamen, statt ihn im Speicher zu halten (`main.go` sagt,
  wo).
- Einen Download-Zähler hinzufügen, in DynamoDB; auch das emuliert Ihr Cloud-Stack.
- Die Dateien hinter eine Anmeldung legen, sodass die Tabelle pro Person gilt.
- Eine Bildvorschau zeigen, mit einer vorsignierten URL, statt die Bytes durchzureichen.

## Lizenz

Dieses Werk steht unter der Lizenz [CC BY-NC-SA 4.0](https://creativecommons.org/licenses/by-nc-sa/4.0/). Sie dürfen es zum Lernen und Unterrichten teilen und bearbeiten, sofern Sie Mitsuru Technologies Ltd nennen, es nicht kommerziell nutzen und alles, was Sie daraus machen, unter derselben Lizenz teilen. Der vollständige Text steht in der Datei `LICENSE`.

## Mehr erfahren

- [Eine App aus GitHub bereitstellen](https://mitsurutech.co.uk/de/help/deploy-app): wie die
  Plattform ein Repository wie dieses ausführt.
- [Der Cloud-Stack](https://mitsurutech.co.uk/de/help/cloud-stack): der S3-artige Dienst, den Ihre
  Sitzung für die App startet.
- Sie unterrichten eine Klasse? [Ihr erstes Jahr ist kostenlos](https://mitsurutech.co.uk/de/pricing) für bis zu
  50 Schülerinnen und Schüler, und für die [Registrierung](https://mitsurutech.co.uk/de/signup) genügt eine E-Mail-Adresse.

## Über dieses Projekt

Erstellt von [Mitsuru Technologies Ltd](https://mitsurutech.co.uk/de/learning) (eingetragen in England und Wales, Registernummer 16755264), dem Unternehmen hinter der
[MitsuruTech Learning Platform](https://mitsurutech.co.uk/de/learning) für Schulen und Colleges.

Dasselbe Beispiel in anderen Sprachen: [Node](https://github.com/mitsurutech/uploads-node),
[Python](https://github.com/mitsurutech/uploads-python) und
[Java](https://github.com/mitsurutech/uploads-java).
