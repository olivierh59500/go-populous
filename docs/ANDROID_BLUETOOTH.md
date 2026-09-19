# Multijoueur Bluetooth sur Android

## Portée actuelle

La version Android propose une partie locale à deux joueurs en **Bluetooth
Classic RFCOMM sécurisé**, sans accès à Internet :

- **HOST / CREATE** joue le camp Good et choisit le monde ;
- **JOIN** joue le camp Evil et reçoit le monde choisi par l'hôte ;
- une seule connexion et une seule partie sont acceptées à la fois ;
- le multijoueur TCP desktop reste disponible et inchangé.

Dans le menu tactile, ouvrir **MULTIPLAYER**, choisir le monde sur le téléphone
hôte, puis toucher **HOST / CREATE**. Sur l'autre téléphone, ouvrir la même
page, toucher **JOIN** et sélectionner l'hôte dans la liste. Le choix de monde
affiché sur le client n'est pas utilisé : l'état initial canonique vient de
l'hôte.

L'hôte doit accepter la visibilité Bluetooth demandée par Android, actuellement
limitée à 180 secondes. Les appareils déjà associés apparaissent avant les
résultats de la recherche. Lors d'une première connexion sécurisée, Android
affiche lui-même la demande d'association sur les deux appareils ; elle doit
être acceptée des deux côtés. Après les 180 secondes, le serveur reste
joignable par un appareil déjà associé, mais n'est plus annoncé aux nouveaux
appareils.

## Architecture

Le transport Android ne duplique pas le protocole du jeu :

```text
moteur Go ↔ TCP 127.0.0.1 ↔ pont Java ↔ RFCOMM sécurisé ↔ pont Java ↔ TCP 127.0.0.1 ↔ moteur Go
```

Le pont copie les octets dans les deux sens. Ses sockets TCP sont liés à la
boucle locale uniquement et le callback Go refuse toute adresse qui n'est pas
une adresse IP numérique de loopback. Aucune interface réseau locale ou distante
n'expose ce relais. Chaque extrémité locale est en plus authentifiée, avant le
handshake du jeu, par une capacité aléatoire de 128 bits à usage unique. Elle
doit être échangée en cinq secondes au plus et est comparée en temps constant.
Les copies binaires mutables conservées par le pont sont remises à zéro après
usage ou annulation. Les runtimes gérés Go et Java ne garantissent toutefois pas
l'effacement physique immédiat de chaque copie immutable en mémoire. Côté client,
le pont attend au plus 30 secondes la connexion du
moteur. La capacité ne traverse jamais RFCOMM et n'est jamais journalisée.

Au-dessus de ce flux, Android réutilise exactement le multijoueur déterministe :
handshake et identifiant de compatibilité, snapshot initial de l'hôte, commandes
ordonnées par l'hôte, ping, hashes d'état tous les 32 tours et snapshot de
resynchronisation. Trois divergences consécutives arrêtent la partie au lieu de
continuer avec deux mondes différents. Deux versions incompatibles sont rejetées
pendant le handshake, avant le début de la simulation.

## Autorisations Android

Le matériel Bluetooth est déclaré facultatif : l'APK peut donc être installé
sur un appareil qui n'en possède pas. L'entrée **MULTIPLAYER** y est désactivée
et une demande directe de création ou de jonction est refusée.

| Version Android | Hôte | Client |
| --- | --- | --- |
| API 23 à 30 | `BLUETOOTH` et `BLUETOOTH_ADMIN` déclarées | identique, plus `ACCESS_FINE_LOCATION` demandée à l'exécution pour la recherche |
| API 31 et plus | `BLUETOOTH_CONNECT` et `BLUETOOTH_ADVERTISE` | `BLUETOOTH_CONNECT` et `BLUETOOTH_SCAN` |

Sur API 31 et plus, `BLUETOOTH_SCAN` porte le drapeau
`neverForLocation`. La permission de localisation historique est plafonnée à
l'API 30 et n'est donc pas demandée sur les Android modernes. Les refus de
permission, d'activation de Bluetooth ou de visibilité doivent ramener une
erreur lisible et laisser une nouvelle tentative possible. Sur certains
appareils API 23–30, la découverte peut aussi nécessiter que le service de
localisation du système soit activé.

Ces choix suivent les recommandations Android sur les
[autorisations Bluetooth](https://developer.android.com/develop/connectivity/bluetooth/bt-permissions),
la [recherche et l'association des appareils](https://developer.android.com/develop/connectivity/bluetooth/find-bluetooth-devices)
et les [connexions RFCOMM serveur/client](https://developer.android.com/develop/connectivity/bluetooth/connect-bluetooth-devices).

## Interruption et reprise

Il n'existe actuellement **ni reconnexion, ni reprise de session**. Une brève
pause causée par une boîte de dialogue Android n'annule pas immédiatement le
transport. En revanche, mettre durablement l'un des jeux en arrière-plan, sortir
de portée, couper Bluetooth, forcer l'arrêt ou détruire l'activité peut fermer la
connexion ou déclencher les délais du lockstep. C'est un arrêt sûr et terminal
de la partie en cours ; les deux joueurs doivent revenir au menu et recommencer
une session.

L'application maintient l'écran allumé lorsqu'elle est visible. Aucun service
Android d'arrière-plan n'est installé pour maintenir artificiellement une
partie suspendue.

## Confidentialité

Le parcours Bluetooth ne demande pas le microphone et ne capture aucun son.
Il ne contacte aucun serveur Internet et ne requiert aucune connexion Internet.
Les octets de jeu sont transmis sur le socket RFCOMM sécurisé puis sur la
boucle locale privée ; le pont ne journalise pas leur contenu.

Ici, « sécurisé » désigne le socket RFCOMM authentifié par l'association
Bluetooth Android. Le protocole Populous n'ajoute ni chiffrement de bout en bout
ni identité de compte : les joueurs doivent donc vérifier le nom et le code
d'association affichés par le système avant de les accepter.

Le nom et l'adresse Bluetooth des appareils peuvent être affichés
temporairement dans le sélecteur Bluetooth de la partie cliente. Le code du pont
ne les enregistre pas dans une sauvegarde ou un journal applicatif. La
permission `INTERNET` reste présente pour le transport TCP partagé par le moteur,
mais le chemin Bluetooth n'ouvre qu'une connexion TCP sur `127.0.0.1`.

## Checklist de validation sur deux appareils

Cette validation nécessite **deux appareils Android physiques** disposant de
Bluetooth Classic. Un seul Pixel était disponible pendant l'implémentation ; une
partie RFCOMM réelle à deux téléphones n'est donc pas encore revendiquée comme
testée.

Sur le Pixel 10a de développement, l'ensemble du parcours à un appareil a été
validé : autorisations Android modernes, visibilité, service RFCOMM réellement
en écoute, fermeture de ce service au retour au titre, scan, affichage des
appareils associés et détectés, annulation puis nouvelle tentative. Aucun crash
ni erreur fatale n'a été relevé. Cette validation ne remplace pas le scénario à
deux appareils ci-dessous.

Préparer le même APK sur les deux appareils, sans supposer qu'ils sont déjà
associés. Pour couvrir les permissions, effectuer si possible une passe sous
API 23–30 et une autre sous API 31 ou plus.

- [ ] Sur l'hôte, choisir un monde identifiable, toucher **HOST / CREATE**,
  accepter les autorisations et la visibilité, puis vérifier l'attente du second
  joueur.
- [ ] Sur le client, toucher **JOIN**, accepter les autorisations, trouver
  l'hôte, le sélectionner et accepter l'association sur les deux appareils.
- [ ] Vérifier que l'hôte joue Good, que le client joue Evil et que les deux
  affichent exactement le monde choisi par l'hôte après le snapshot initial.
- [ ] Jouer plusieurs minutes et envoyer des actions des deux camps : modelage
  du terrain, fanion, tendances et pouvoirs. Vérifier l'ordre identique et
  l'absence de blocage sur les deux écrans.
- [ ] Refaire la connexion avec deux appareils déjà associés ; l'hôte doit
  apparaître immédiatement dans la liste avec l'indication `[PAIRED]`.
- [ ] Refuser tour à tour les permissions de l'hôte et du client. Vérifier un
  retour propre au menu Bluetooth, puis une nouvelle tentative après accord.
- [ ] Refuser l'activation de Bluetooth, la visibilité et la boîte d'association,
  puis annuler le sélecteur d'appareil. Chaque cas doit être annulable et
  retentable sans redémarrer l'application.
- [ ] Sélectionner un appareil qui n'expose pas le service Populous, ou lancer
  une mauvaise application sur l'appareil distant. La recherche SDP/RFCOMM doit
  échouer proprement, sans commencer une partie.
- [ ] Essayer deux builds ayant des identifiants de compatibilité différents.
  Le handshake doit refuser la session sur les deux côtés avant toute
  simulation.
- [ ] Pendant une partie, couper Bluetooth, éloigner un appareil, tuer
  l'application puis détruire son activité. Le pair restant doit constater la
  déconnexion ou un timeout et ne doit jamais continuer une simulation locale
  divergente.
- [ ] Mettre un appareil en arrière-plan assez longtemps pour dépasser les
  délais réseau. Vérifier l'arrêt de la session et l'absence de reconnexion
  implicite. Refaire aussi une pause très courte due au dialogue d'association,
  qui ne doit pas annuler la préparation de la connexion.
- [ ] Avec un build de diagnostic, injecter une divergence contrôlée d'abord
  dans le monde Good, puis dans le monde Evil. Dans les deux sens, vérifier les
  hashes au prochain multiple de 32 ticks, l'envoi du snapshot par l'hôte et la
  convergence après restauration. Une divergence maintenue trois fois doit
  fermer la session.
- [ ] Contrôler `logcat` et les fichiers privés après une partie : aucun contenu
  des paquets, snapshot ou commande ne doit être journalisé. Vérifier aussi
  l'absence de permission microphone et que la partie fonctionne sans Wi-Fi ni
  données mobiles.

## Couverture automatisée auditée

La commande suivante passe sur le poste de développement :

```sh
go test ./internal/platformbridge ./internal/mobileui ./internal/multiplayer
go test -race ./internal/platformbridge ./internal/multiplayer
```

Elle couvre la concurrence entre callbacks Android et boucle du jeu, les
annulations et callbacks tardifs, la limite de file, la copie et l'effacement de
la capacité locale, le refus d'un proxy hors loopback, le nettoyage des textes
distants, les cibles tactiles du menu, l'authentification correcte, erronée,
expirée ou surdimensionnée du proxy, le handshake et les versions incompatibles,
l'ordre lockstep, les snapshots et la resynchronisation, les hashes, le ping, la
déconnexion, ainsi que les limites de taille et de décompression du protocole.

Ces tests utilisent des connexions en mémoire ou TCP local. Ils ne peuvent pas
valider l'adaptateur Android, les boîtes de permission et d'association, la
découverte radio, la qualité du lien RFCOMM ou le cycle de vie de deux appareils.
Ils ne pilotent pas non plus une instance graphique de `Game` pour vérifier le
retour au menu après chaque erreur, ni une divergence de monde de bout en bout
jusqu'au troisième hash. La checklist physique ci-dessus reste donc obligatoire
avant d'annoncer la fonction comme validée sur le matériel ciblé.
