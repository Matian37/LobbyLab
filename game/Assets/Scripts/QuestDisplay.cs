using System.Collections;
using System.Collections.Generic;
using UnityEngine;
using TMPro;
using UnityEngine.UI;

public class QuestDisplay : MonoBehaviour
{
    public TextMeshProUGUI[] quest, rew;
    public GameObject[] collected, collect;
    public Color collectColor, defColor;
    private void Start()
    {
        Refresh();
    }

    public void Refresh()
    {
        for (int i = 0; i < 2; i++)
        {
            if (Quests.Instance.collected[i])
            {
                collected[i].SetActive(true);
                quest[i].gameObject.SetActive(false);
                collect[i].SetActive(false);
                rew[i].gameObject.SetActive(false);
                continue;
            }
            else if (Quests.Instance.collect[i])
            {
                collected[i].SetActive(false);
                quest[i].gameObject.SetActive(false);
                collect[i].SetActive(true);
                quest[i].transform.parent.GetComponent<Button>().enabled = true;
                quest[i].transform.parent.GetComponent<Image>().color = collectColor;
                rew[i].text = Quests.Instance.rewards[Quests.Instance.ind[i]].ToString();
                continue;
            }
            quest[i].text = Quests.Instance.quests[Quests.Instance.ind[i]];
            rew[i].text = Quests.Instance.rewards[Quests.Instance.ind[i]].ToString();
        }
    }

    public void Collect(int indx)
    {
        SoundManager.Instance.Sfx(SoundManager.Instance.collect);
        Quests.Instance.collected[indx] = true;
        Quests.Instance.collect[indx] = false;
        collect[indx].SetActive(false);
        collected[indx].SetActive(true);
        SceneTransport.Instance.xp += Quests.Instance.rewards[Quests.Instance.ind[indx]];
        quest[indx].transform.parent.GetComponent<Image>().color = defColor;
        quest[indx].transform.parent.GetComponent<Button>().enabled = false;
        FindObjectOfType<MenuManager>().xpText.text = SceneTransport.Instance.xp.ToString();
        FindObjectOfType<MenuManager>().odblText.text = SceneTransport.Instance.odb.ToString();
        Refresh();
        SceneTransport.Instance.Save();
    }
}
