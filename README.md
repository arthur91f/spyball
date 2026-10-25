# Spyball

Jeu d'ambiance pour soirée (anniversaire, Noël, nouvel an…), joué en parallèle de la soirée grâce à un petit site web
local. Chaque invité découvre sur son téléphone un rôle secret : un personnage, un camp, un signe d'appel et quelques
indices. Pendant la soirée, il cherche discrètement ses alliés et démasque ses ennemis, et note ses suppositions sur le
site. À la fin, l'organisateur clôture la partie : les rôles sont dévoilés et les points calculés.

Le thème d'origine, « Le bal des nations », met en scène un gala d'ONG infiltré par les espions des démocraties, des
puissances autoritaires et de quelques électrons libres. Les règles complètes sont sur la page **Règles** du site.

## Lancer le serveur

Prérequis : [Go](https://go.dev/dl/) 1.27 ou plus récent.

```bash
SPYBALL_ADMIN_PASSWORD=monmotdepasse go run ./cmd/spyball
```

Au démarrage, le serveur affiche les adresses à partager aux invités, par exemple :

```
INFO players can join at url=http://192.168.1.42:8080
```

Les téléphones doivent être sur le même réseau Wi-Fi que l'ordinateur qui fait tourner le serveur.

Pour fabriquer un binaire autonome (thèmes, pages et styles sont inclus dedans) :

```bash
go build -o spyball ./cmd/spyball
```

Options utiles (`./spyball -h` pour la liste complète) :

| Option | Par défaut | Rôle |
|--------|------------|------|
| `-addr` | `:8080` | adresse et port d'écoute |
| `-admin-password` | `$SPYBALL_ADMIN_PASSWORD` | mot de passe de la page admin ; s'il est absent, un mot de passe aléatoire est affiché au démarrage |
| `-data` | `data` | dossier où sont enregistrées la partie (`game.json`) et la clé des sessions (`session.key`) |
| `-content` | contenu intégré | dossier contenant `themes/`, `setups/` et `locales/`, pour utiliser des thèmes modifiés sans recompiler |
| `-lang` | `fr` | langue de l'interface (un fichier de `locales/`) |

La partie est enregistrée à chaque action : on peut arrêter et relancer le serveur sans rien perdre, et les joueurs
restent connectés. Pour repartir de zéro, utiliser « Commencer une nouvelle partie » sur la page admin (ou supprimer
`data/game.json`).

## Déroulement d'une soirée

1. **L'organisateur** ouvre `/admin`, saisit la liste des invités (un nom par ligne), choisit le thème et la
   configuration adaptée au nombre de joueurs, puis lance le tirage au sort. Le tirage étant aléatoire, il peut jouer
   lui aussi : la page admin ne montre jamais les rôles.
2. **Chaque invité** ouvre le site sur son téléphone, choisit son nom et découvre son personnage. Ce personnage lui sert
   ensuite de mot de passe pour se reconnecter (majuscules, accents et ponctuation sont ignorés). En cas d'oubli,
   l'organisateur peut réinitialiser sa connexion depuis la page admin.
3. **Pendant la soirée**, chacun fait son signe, observe les autres et indique sur sa page qui il pense être allié ou
   ennemi. Les non-alignés choisissent leur camp.
4. **À la fin**, l'organisateur clôture la partie. La page **Résultats** dévoile les rôles, les suppositions et le
   classement. L'organisateur peut y ajouter des points : défis réussis, dénonciations publiques…

## Ajouter un thème

Un thème est un fichier `themes/<id>/theme.yaml`. Il ne demande aucune modification du code : il donne un nom aux
camps, des personnages aux emplacements des configurations et fournit les signes. Le fichier
[`themes/ball-of-nations/theme.yaml`](themes/ball-of-nations/theme.yaml) est commenté ; voici un thème de Noël complet,
utilisable avec la configuration à 12 joueurs :

```yaml
name: La nuit de Noël
story: >-
  Cette nuit, les lutins du père Noël doivent livrer les cadeaux, mais les disciples du père Fouettard
  se sont glissés parmi eux pour tout saboter.

teams:
  blue: {name: Lutins du père Noël, member: lutin, description: Ils se reconnaissent à leur façon de parler.}
  red: {name: Disciples du père Fouettard, member: disciple, description: Ils se reconnaissent par gestes.}
  neutral: {name: Habitants du pôle Nord, member: habitant}

characters:            # l'emplacement blue-1 reçoit le premier personnage bleu, etc.
  blue: [{name: Lutin farceur}, {name: Lutin cuisinier}, {name: Lutin postier}, {name: Lutin menuisier}]
  red: [{name: Fouettard}, {name: Krampus}, {name: Croque-mitaine}, {name: Grinch}]
  white: [{name: Rudolph}, {name: Bonhomme de neige}]
  yellow:              # les jaunes sont choisis selon leur règle
    - {name: Mère Noël, rule: ally_of_everyone, description: Tout le monde a intérêt à te désigner comme alliée.}
    - {name: Ours polaire, rule: enemy_of_everyone, description: Tout le monde a intérêt à te désigner comme ennemi.}

signs:                 # les blancs tirent un signe bleu et un signe rouge : prévoir assez de signes
  blue: [chanter, dire « ho ho ho », parler du temps qu'il fait, faire des rimes, citer un film de Noël, dire « en vrai »]
  red: [se frotter les mains, croiser les bras, cligner des yeux, se gratter le nez, taper du pied, hausser les épaules]
  yellow: ["faire chanter un {red}", "faire danser un {blue}"]   # {red}, {blue}, {neutral} : le « member » du camp
```

Au démarrage, le serveur vérifie tous les fichiers et refuse de démarrer en listant les erreurs : champ inconnu, nom de
personnage en double, pas assez de personnages ou de signes pour une configuration… Pour essayer un thème sans
recompiler, utiliser `-content` sur une copie des dossiers `themes/`, `setups/` et `locales/`.

## Ajouter une configuration

Une configuration (`setups/<id>.yaml`) décrit une partie pour un **nombre exact** de joueurs, indépendamment du thème :
les emplacements (`blue-1`, `red-2`, `white-1`, `yellow-1`…) et les indices (qui connaît le signe de qui). Voir
[`setups/12-classic.yaml`](setups/12-classic.yaml). Il faut autant de `blue` que de `red`, entre 6 et 24 joueurs, et une
règle (`ally_of_everyone` ou `enemy_of_everyone`) pour chaque `yellow`. Pour une autre taille de groupe, copier une
configuration et retirer ou ajouter des neutres.

## Ajouter une langue

Copier `locales/fr.yaml` en `locales/<langue>.yaml`, traduire les textes, puis lancer le serveur avec `-lang <langue>`.
Toute clé manquante est détectée : la page concernée affiche une erreur au lieu d'un texte vide.

## Développement

```bash
go test ./...         # tests, dont une partie complète jouée de bout en bout
go test -race ./...   # avec détection des accès concurrents
go vet ./...
```

L'organisation du code et les conventions sont décrites dans [`CLAUDE.md`](CLAUDE.md).
