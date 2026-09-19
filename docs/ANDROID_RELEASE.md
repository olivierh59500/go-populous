# APK Android distribuable

Le script construit un APK ARM64 signé pour une installation directe depuis un
lien de téléchargement. Il n'installe rien sur le Pixel et ne nécessite pas de
certificat émis par Google : la clé locale signe un certificat d'application
auto-signé, conservé pour les futures mises à jour.

## Construction

Prérequis identiques au développement Android : Go compatible avec `go.mod`,
Java 17, SDK Platform 36, Build Tools 36.0.0, NDK 28.2.13676358, Gradle 8.11.1
via le wrapper et Ebitengine/ebitenmobile 2.9.11. Le script accepte `ANDROID_HOME`
ou `ANDROID_SDK_ROOT`, ainsi que `JAVA_HOME`. Le NDK choisi est explicitement
transmis à gomobile. `openssl`, `unzip` et `shasum` doivent être disponibles.

Première construction, avec création explicite d'une clé :

```sh
./scripts/build-android-release.sh --init-key
```

Constructions suivantes, avec la même clé :

```sh
./scripts/build-android-release.sh
```

Ne pas exécuter simultanément ce script et `scripts/run-android.sh` : tous deux
régénèrent `android/app/libs/populous.aar`. La configuration release applique
`-trimpath -ldflags '-s -w'` au moteur Go, R8 au code Java et la suppression des
ressources inutilisées. Les noms et callbacks Java utilisés par Go/JNI sont
préservés. Seule l'architecture `arm64-v8a` est incluse.

La version actuelle est `1.1.0`, code `3`, définie dans
`android/app/build.gradle`. Incrémenter le code à chaque version distribuée.

Les fichiers de publication sont générés dans `dist/android/`. L'APK courant et
`SHA256SUMS` sont explicitement inclus dans Git ; les clés, certificats de
travail, rapports et archives locales restent ignorés :

- `populous-android-1.1.0-arm64.apk` : l'application signée ;
- `SHA256SUMS` : empreinte du fichier à télécharger ;
- `SIGNATURE.txt` et `populous-release-cert.pem` : certificat public et vérification ;
- `RELEASE.txt` : versions des outils et propriétés de cette construction.

Le script ne remplace le fichier distribuable qu'après les vérifications. Si un
ancien APK du même nom a un contenu différent, il en conserve une copie dans un
sous-dossier `previous.*`. La procédure fixe les outils, sans promettre que deux
constructions produiront un APK identique octet par octet.

## Clé et mises à jour

`android/keystore/` est ignoré par Git et privé (`0700`). Il contient la clé RSA
3072 bits (`populous-release.p12`) et son mot de passe aléatoire
(`populous-release.password`), chacun en `0600`. Les mots de passe passent aux
outils par fichier et ne sont pas imprimés. Le certificat est valable 50 ans
avec le nom `CN=Populous Android`.

Sauvegarder ce dossier dans un emplacement privé et durable avant diffusion.
Le script réutilise toujours la clé existante et refuse de remplacer une paire
clé/mot de passe incomplète. **Ne distribuer ni la clé ni le fichier de mot de
passe.** La mise à jour d'une application installée nécessite une signature
compatible ; les détails sont décrits dans la
[documentation Android sur la signature](https://developer.android.com/studio/publish/app-signing).

L'APK release ne peut pas remplacer directement l'application debug signée par
une autre clé, même si son numéro de version est supérieur. Le script ne
désinstalle donc jamais la version du Pixel et ne touche pas à sa sauvegarde.
Une éventuelle migration de cette installation de développement doit préserver
les données séparément ; les prochaines mises à jour release réutiliseront la
clé release.

## Téléchargement et compatibilité

Héberger l'APK sur un serveur HTTPS et fournir un lien vers le fichier, avec son
empreinte SHA-256. Android peut demander d'autoriser l'installation d'applications
pour le navigateur ou le gestionnaire de fichiers utilisé. Le certificat local
ne remplace pas les contrôles affichés par Android.

La signature technique et la vérification d'identité du développeur sont deux
sujets distincts. Au 19 septembre 2026, Google annonce une application régionale
de sa nouvelle vérification à compter du 30 septembre 2026 pour certains magasins,
puis une extension en 2027. Les conditions doivent donc être revérifiées avant
une diffusion plus large : [vérification Android des développeurs](https://developer.android.com/developer-verification).
Cette construction n'inscrit aucun compte et ne prétend pas contourner ces règles.

L'APK cible Android API 36, accepte API 23 et plus, et contient du code ARM64.
Cette configuration vise les Pixel récents ; le Pixel 10a est le matériel de
test du projet. Elle n'est pas une validation matérielle de chaque Pixel 8, 9,
10 ou 11, ni une garantie sur une version future d'Android.

La construction vérifie la signature avec
[`apksigner`](https://developer.android.com/tools/apksigner), l'alignement ZIP
16 Kio et **tous** les segments ELF `LOAD`/`GNU_RELRO` de chaque bibliothèque
native. Les options natives `max-page-size=16384` et `common-page-size=16384`
sont transmises par `CGO_LDFLAGS`, y compris pour l'alignement de fin `GNU_RELRO`.
Les bibliothèques restent non compressées afin d'être chargées
directement depuis l'APK. L'alignement statique ne remplace pas un test de jeu
sur un appareil configuré avec des pages mémoire de 16 Kio ; voir le
[guide Android 16 Kio](https://developer.android.com/guide/practices/page-sizes).

L'APK embarque les ressources historiques du projet. La construction et la
signature ne constituent pas une autorisation de redistribuer ces ressources.

## Validation du 19 septembre 2026

La version `1.0.0 (2)` a été lancée sur le Pixel 10a sous Android 17 : écran
d'introduction, menus, début de partie, déplacement à deux doigts, panneau des
pouvoirs et chargement de la sauvegarde existante fonctionnels. Pour préserver
l'installation de développement et ses données, ce test utilise la signature
debug déjà présente sur le téléphone, mais **le même contenu release** : les
13 entrées applicatives (manifeste, DEX, bibliothèque native, ressources et
métadonnées hors signature) ont été comparées octet par octet à l'APK public.
L'application testée reste non débogable et optimisée par R8.

L'APK dans `dist/android/` conserve sa signature release privée distincte.
Le Pixel testé utilise des pages de 4 Kio : le support 16 Kio a été contrôlé
dans l'APK et les ELF, mais n'a pas été exécuté sur un appareil en mode 16 Kio.

La version `1.1.0 (3)` ajoute le multijoueur local Bluetooth. Le build debug
correspondant a été installé sur le Pixel 10a : permission « appareils à
proximité », visibilité, enregistrement du service RFCOMM sécurisé, attente de
l'hôte, fermeture du service, recherche cliente, appareils déjà associés et
annulation/reprise du sélecteur sont validés. La sauvegarde et les préférences
ont conservé leurs empreintes. Une partie complète reste à vérifier avec un
second appareil physique ; voir le [guide Bluetooth](ANDROID_BLUETOOTH.md).

Le contenu release optimisé a également été resigné temporairement avec la clé
debug déjà installée, afin de le lancer sans désinstaller l'application ni perdre
ses données. Ses 11 entrées applicatives hors signature correspondent octet par
octet à l'APK public ; l'application non débogable démarre et rend le nouveau
menu. Le build debug final a ensuite été remis sur le Pixel et les deux empreintes
de données ont de nouveau été contrôlées. La copie temporaire resignée a été
supprimée.
