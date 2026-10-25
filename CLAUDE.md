## Description générale
Je voudrais faire un jeux d'ambiance pour une soirée. L'idée du jeu est de découvrir qui est dans son équipe et qui est dans l'équipe adverse, avec certains joueur qui endosseront un role neutre asymétrique. L'idée est qu'au début de la soirée, chacun puisse se connecter au serveur et découvrir son role et qu'au cours du jeu chacun puisse entrer ses suppositions. A la fin de la soirée les roles des joueurs sont dévoilés et les points de chacun calculé.

## Les pages à écrire
Il faudra donc :
- une page de login
- une page affichant le role du joueur et permettant d'entrer les suppositons du joueurs et d'afficher/modifier les suppositions déjà entrée.
- une page affichant les résultats et les suppositions de chacun accessible uniquement à la fin de la soirée

En plus il faudra:
- une page admin pour entrer la liste des joueurs, définir les paramètres de la partie et terminer le jeu pour cloturer l'entrer des suppositions par les joueurs et permettre l'accès à la page des résultats.
- une page affichant les règles


## A titre informatif
Des brouillons d'implémentation non définitive ont été écris dans le dossier old-games-drafts. Cela pourra-être utile pour affiner le fichier d'instruction général et je pense que ça pourra être ignoré après.

## Les règles du jeu:
Elles sont globalement décrites dans old-game-drafts/jeux-nouvel-an. Cependant ce jeux était en format papier et ici nous voulons le refaire à l'aide d'un site web et le rendre adaptable aux nombres de joueurs. Il était aussi spécifique à une soirée de nouvel an alors que le jeu devra pouvoir être joué lors de n'importe quel soirée ou goûter d'anniversaire. Il y a donc des adaptation à faire.

## Perspectives futur
Le jeux a été pensé avec pour thème d'un bal donnée par une ONG auxquels se joignent des espions de toute les nations. Les équipes était donc les démocratie du monde libres contre les puissances autoritaires et des faction non aligné ayant un rôle asymétrique. Néanmoins le thème doit resté au second plan et doit pouvoir être changé facilement. Par exemple les lutins du père Noël contre les disciples du père fouettard pour un thème convenant plus à une soirée de Noël, ou les sorciers contre fantômes pour un thème plus Halloween. Dans un premier temps, on se concentrera sur l'implémentation du thème original mais il faudra qu'on puisse ajouter un autre thème facilement. Pour le code on se contentera de parler d'équipe bleue et rouge et éventuellement d'équipe jaune (les rôles asymétrique) ou blanc pour des rôles pouvant choisir entre rouge et bleu.

## Adaptation
J'ai écris quelques configurations de jeu pour un nombre finit de joueur. Il serait bien qu'on puisse dans un second temps trouver au moins une configuration équilibré pour des groupes de joueurs de tout nombre de 6 à 24 joueurs.

## Précision techniques
Ecris le site en go principalement, css et html dans un git.
Le code doit être en anglais.
Dans un premier temps l'interface sera en français mais on pourra ajouter d'autre langue dans un second temps.
Le thème, le rôle des joueurs et les signes distinctifs pourront être ajouter dans un dossier distinct en yaml.
Les différentes configuration pour différent nombres de joueurs seront aussi dans un dossier distinct.
Dans un premier temps il nous faudra une configuration de jeux pour un nombre de joueur définit et un thème dans un dossier.



