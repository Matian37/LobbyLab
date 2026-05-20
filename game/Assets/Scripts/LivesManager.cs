using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class LivesManager : MonoBehaviour
{
    public int lives = 3;
    public GameManager gen;
    public bool isLeft;
    public GameObject[] livesImg;
    public float timeToRegen = 20;
    float time = 0;
    public void EraseLife()
    {
        time = 0;
        lives--;
        SkyBoxManager.Instance.ChangeSky(SkyBoxManager.Instance.red, true, 4, true, false);
	livesImg[lives].SetActive(false);
        if (lives == 0)
        {
            gen.Lose(!isLeft);
        }
    }

    public void Update()
    {
        time += Time.deltaTime;
        if (time >= timeToRegen)
        {
	    if(lives < 3) livesImg[lives].SetActive(true);
            lives = Mathf.Min(lives + 1, 3);
            time = 0;
        }
    }
}
