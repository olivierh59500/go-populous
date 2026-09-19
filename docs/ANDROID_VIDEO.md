# Présentation Android enregistrée sur le Pixel

Fichier livré : `recordings/populous-android-presentation.mp4`, environ 4 min 55,
dont quatre minutes continues de partie IA. Résolution native 2424 × 1080,
30 images/seconde, H.264 et son AAC stéréo, chapitres intégrés. Le film final
commence directement sur l'illustration originale ; les images du lanceur
Android et de la transition système ont été exclues. Ne partager que ce MP4
final : les prises brutes de travail restent locales et privées.

La capture est effectuée sur le Pixel 10a connecté, dans l'application Android
réelle. Aucun rendu desktop n'est présenté comme une capture du téléphone.
Le lancement montre l'illustration historique pendant trois secondes, puis le
menu tactile. La partie utilise le mode existant **PPC VS PPC** du menu
**GAME SETUP** : deux IA historiques jouent à vitesse normale dans l'interface
mobile, sans bonus de mana, de population ou de temps.

Monde choisi pour la séquence : **72 — EOAEING**. Les camps sont proches, ce qui
rend les constructions et combats visibles dans une même vue. Le mode démo
séparé, avec sa présentation à deux vues et son IA stratégique, n'est pas modifié.

## Son de l'application uniquement

`screenrecord` capture les images ; le microphone et les pistes audio globales
du téléphone ou du Mac ne sont jamais enregistrés. Pour la bande-son, un mode
explicitement activé dans la version de développement journalise les sons
réellement déclenchés : horodatage, identifiant, volume et arrêt d'une voix.
L'outil `cmd/audio-trace` reconstruit ensuite le WAV à partir des échantillons
originaux du jeu. Il conserve les sons commencés avant le début du découpage.

L'activation doit précéder le premier `Update` du moteur. Exemple de lancement
ADB avec un **nouveau** nom de trace (un fichier existant n'est jamais écrasé) :

```sh
adb shell am start -S -n com.olivierh.populous/.MainActivity \
  --es capture_audio presentation-unique.jsonl
```

Le nom est validé et la trace reste dans le répertoire privé de Populous. Seule
une application `debuggable` accepte cette option ; elle est inactive dans l'APK
release distribué aux testeurs. Ni permission microphone, ni permission de
capture audio système, ni écriture dans un stockage partagé ne sont ajoutées.

Récupération pendant que l'application continue de fonctionner :

```sh
adb exec-out run-as com.olivierh.populous cat files/presentation-unique.jsonl > audio.jsonl
go run ./cmd/audio-trace -trace audio.jsonl -output audio.wav \
  -start-ns DEBUT_VIDEO_UNIX_NANOSECONDES -duration 4m45s
```

Le timestamp est celui du téléphone, pas celui du Mac. Le décalage de découpage
vidéo doit être également appliqué au début du WAV. Le MP4 final ne conserve
que la vidéo du jeu et le son reconstruit, sans les pistes techniques du téléphone.

Sur le Pixel utilisé, `screenrecord` fournit une piste technique Winscope v2
(`0:2`) avec l'horodatage précis de chaque image. Elle évite d'utiliser comme
origine l'heure du lancement de la commande, qui précède la première image de
plusieurs centaines de millisecondes. Le format et son calcul d'offset sont
décrits dans le [code AOSP de screenrecord](https://android.googlesource.com/platform/frameworks/av/+/572778c8ee62a602faaf69554dc1acd28f5902d2/cmds/screenrecord/screenrecord.cpp#433).
Pour extraire le début exact d'une capture de cette version :

```sh
ffmpeg -v error -i raw.mp4 -map 0:2 -c copy -f data - | perl -0777 -ne '
  die "Not Winscope metadata v2\n" unless length($_)>=40 && substr($_,0,16) eq "#VV1NSC0PET1ME2#";
  my($v,$offset,$count,$first)=unpack("L<q<L<Q<",substr($_,16,24));
  die "Invalid metadata\n" unless $v==2 && $count>0 && length($_)==32+8*$count;
  printf "%d\n",$offset+$first;'
```

Cette méthode suppose que l'horloge système ne subit pas de correction brutale
pendant la capture. Le WAV reproduit les sons à leur déclenchement logiciel,
sans ajouter la latence matérielle du haut-parleur du téléphone.

Les captures et fichiers intermédiaires restent dans `recordings/`, ignoré par
Git. Les sauvegardes du joueur et les préférences tactiles sont vérifiées par
empreinte avant et après le parcours ; aucun bouton de sauvegarde n'est utilisé.

## Contrôles

Les tests unitaires couvrent le mélange PCM, les volumes, la saturation, les
arrêts explicites, les queues de sons, la lecture d'une trace encore ouverte et
le refus d'écraser des fichiers. La reconstruction est effectuée par blocs,
sans allouer un tampon de la durée totale de la vidéo.
