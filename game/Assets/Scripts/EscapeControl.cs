using System.Collections;
using System.Collections.Generic;
using UnityEngine;
using UnityEngine.SceneManagement;  

public class EscapeControl : MonoBehaviour
{
    public GameObject escape, settings, wholeEscape;
    bool act = false;

    private void Update()
    {
        if(Input.GetKeyDown(KeyCode.Escape) && settings.activeInHierarchy)
        {
            SettingsSwitch(false);
        }
        else if (Input.GetKeyDown(KeyCode.Escape))
        {
            EscapeSwitch(!act);
        }
    }

    public void EscapeSwitch(bool x)
    {
        if(x) SoundManager.Instance.Sfx(SoundManager.Instance.pause);
        else SoundManager.Instance.Sfx(SoundManager.Instance.unpause);
        act = x;
        wholeEscape.SetActive(x);
        escape.SetActive(x);
        SoundManager.Instance.Pause(x);
        if (act) Time.timeScale = 0;
        else Time.timeScale = 1;
    }

    public void SettingsSwitch(bool x)
    {
        SoundManager.Instance.Sfx(SoundManager.Instance.click);
        settings.SetActive(x);
        escape.SetActive(!x);
    }

    public void MenuButton()
    {
        SoundManager.Instance.Sfx(SoundManager.Instance.click);
        foreach (var gen in FindObjectsOfType<GameManager>())
        {
            if(gen.gameObject.activeInHierarchy)
            {
                gen.Save();
            }
        }
        SceneManager.LoadScene("Menu");
    }

    public void AgainButton()
    {
        SoundManager.Instance.Sfx(SoundManager.Instance.click);
        SceneManager.LoadScene("SampleScene");
    }
}
