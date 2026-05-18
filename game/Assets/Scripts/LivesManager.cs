using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class LivesManager : MonoBehaviour
{
    public int lives = 3;
    public ObstaclesGen gen;
    public bool isLeft;
    public GameObject[] livesImg;
    public float timeToRegen = 20;
    float time = 0;
    public void EraseLife()
    {
        time = 0;
        lives--;
        SkyBoxManager.Instance.ChangeSky(SkyBoxManager.Instance.red, true, 4, true, false);
        Destroy(livesImg[lives]);
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
            lives = Mathf.Min(lives + 1, 3);
            time = 0;
        }
    }
}
